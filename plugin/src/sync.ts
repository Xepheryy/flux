import { Notice, requestUrl, TFile } from "obsidian";
import type { TAbstractFile, Vault } from "obsidian";
import type { FluxSettings } from "./settings";

const ORIGIN_FLUX = "flux";
const PUSH_DEBOUNCE_MS = 500;

function contentHash(str: string): string {
  let h = 0;
  for (let i = 0; i < str.length; i++) h = ((h << 5) - h + str.charCodeAt(i)) | 0;
  return `sha256:${Math.abs(h).toString(36)}${str.length}`;
}

function errorMessage(e: unknown): string {
  const err = e instanceof Error ? e : new Error(String(e));
  const cause = (err as Error & { cause?: unknown }).cause;
  return cause instanceof Error ? cause.message : err.message;
}

export interface PushFile {
  path: string;
  content: string;
  hash: string;
}

export interface PullFile {
  path: string;
  content: string;
  hash: string;
}

export interface PullResponse {
  files: PullFile[];
  deleted: string[];
}

export class FluxSync {
  private settings: FluxSettings;
  private vault: Vault;
  private pushDebounce = new Map<string, ReturnType<typeof setTimeout>>();
  private applyingPull = false;
  private abortController: AbortController | null = null;

  constructor(settings: FluxSettings, vault: Vault) {
    this.settings = settings;
    this.vault = vault;
  }

  updateSettings(settings: FluxSettings) {
    this.settings = settings;
  }

  private get baseUrl(): string {
    let u = this.settings.endpoint.trim();
    if (!u) return "";
    if (!u.startsWith("http://") && !u.startsWith("https://")) {
      const isLocal = u.startsWith("localhost") || u.startsWith("127.0.0.1");
      u = `${isLocal ? "http" : "https"}://${u}`;
    }
    return u.replace(/\/$/, "");
  }

  /** Creates parent folder and all ancestors; no-op if dir is empty or already exists. */
  private async ensureParentFolders(filePath: string): Promise<void> {
    const dir = filePath.includes("/") ? filePath.replace(/\/[^/]+$/, "") : "";
    if (!dir) return;
    const parts = dir.split("/").filter(Boolean);
    let acc = "";
    for (const part of parts) {
      acc = acc ? `${acc}/${part}` : part;
      if (!this.vault.getAbstractFileByPath(acc)) await this.vault.createFolder(acc);
    }
  }

  /** Uses requestUrl for mobile compatibility (bypasses CORS). */
  private async api(path: string, opts: { method?: string; body?: string } = {}): Promise<{ ok: boolean; status: number; json: () => Promise<unknown> }> {
    if (!this.baseUrl) throw new Error("Flux endpoint not configured");
    const headers: Record<string, string> = { "Content-Type": "application/json" };
    const { username, password } = this.settings;
    if (username || password) {
      headers.Authorization = `Basic ${btoa(`${username}:${password}`)}`;
    }
    const url = this.baseUrl + path;
    const res = await requestUrl({
      url,
      method: opts.method ?? "GET",
      body: opts.body,
      headers,
    });
    const json = async (): Promise<unknown> => {
      const j = res.json;
      if (typeof j === "function") {
        const out = (j as () => unknown)();
        return out instanceof Promise ? out : Promise.resolve(out);
      }
      if (j != null && typeof (j as Promise<unknown>).then === "function") return j as Promise<unknown>;
      if (typeof j === "string") return JSON.parse(j);
      return j;
    };
    return {
      ok: res.status >= 200 && res.status < 300,
      status: res.status,
      json,
    };
  }

  async pushFile(file: TFile): Promise<void> {
    if (this.applyingPull || !this.settings.enabled || !this.baseUrl || file.extension !== "md") return;

    const path = file.path;
    const t = this.pushDebounce.get(path);
    if (t) clearTimeout(t);
    this.pushDebounce.set(
      path,
      setTimeout(async () => {
        this.pushDebounce.delete(path);
        try {
          const content = await this.vault.read(file);
          const res = await this.api("/push", {
            method: "POST",
            body: JSON.stringify({ files: [{ path, content, hash: contentHash(content) }] }),
          });
          if (!res.ok) throw new Error(`Push failed: ${res.status}`);
          new Notice(`Flux: pushed ${path}`);
        } catch (e) {
          console.error("[Flux] push error:", e);
          new Notice(`Flux: push failed — ${errorMessage(e)}`);
        }
      }, PUSH_DEBOUNCE_MS)
    );
  }

  async pushFiles(files: TFile[]): Promise<void> {
    for (const f of files) await this.pushFile(f);
  }

  /** Push multiple files in one request and await (for initial sync on enable). */
  async pushAllNow(files: TFile[]): Promise<void> {
    if (!this.settings.enabled || !this.baseUrl || this.applyingPull) return;
    const pushFiles: PushFile[] = [];
    for (const f of files) {
      if (f.extension !== "md") continue;
      const content = await this.vault.read(f);
      pushFiles.push({ path: f.path, content, hash: contentHash(content) });
    }
    if (pushFiles.length === 0) return;
    const res = await this.api("/push", {
      method: "POST",
      body: JSON.stringify({ files: pushFiles, deleted: [] }),
    });
    if (!res.ok) throw new Error(`Push failed: ${res.status}`);
  }

  async pull(): Promise<void> {
    if (!this.settings.enabled || !this.baseUrl) return;
    this.abortController?.abort();
    this.abortController = new AbortController();
    try {
      const res = await this.api("/pull", { method: "GET" });
      const data = (await res.json()) as PullResponse;
      if (!res.ok) {
        const msg = typeof data === "object" && data != null && "error" in data ? String((data as { error?: string }).error) : "";
        throw new Error(`Pull failed: ${res.status}${msg ? ` — ${msg}` : ""}`);
      }
      this.applyingPull = true;
      let applied = 0;
      let removed = 0;
      const filesList = Array.isArray((data as PullResponse).files) ? (data as PullResponse).files : [];
      console.log("[Flux] pull response:", filesList.length, "files,", (data.deleted?.length ?? 0), "deleted");
      try {
        for (const raw of data.deleted || []) {
          const p = typeof raw === "string" ? raw.trim().replace(/\\/g, "/") : "";
          if (!p) continue;
          const f = this.vault.getAbstractFileByPath(p);
          if (f) {
            await this.vault.delete(f);
            removed++;
          }
        }
        // Apply all files from server (whole vault sync; no path filter)
        for (const f of filesList) {
          if (!f || typeof f.path !== "string" || typeof f.content !== "string") {
            continue;
          }
          const path = (f.path as string).trim().replace(/\\/g, "/");
          const existing = this.vault.getAbstractFileByPath(path);
          if (existing && existing instanceof TFile) {
            const cur = await this.vault.read(existing);
            if (contentHash(cur) !== f.hash) {
              await this.vault.modify(existing, f.content, {});
              applied++;
            }
          } else {
            await this.ensureParentFolders(path);
            await this.vault.create(path, f.content, {});
            applied++;
          }
        }
      } finally {
        this.applyingPull = false;
      }
      // Diff: push local-only files so server tree matches device
      const remotePaths = new Set(filesList.map((f) => (f.path as string).trim().replace(/\\/g, "/")));
      const deletedSet = new Set((data.deleted || []).map((p) => (typeof p === "string" ? p.trim().replace(/\\/g, "/") : "")).filter(Boolean));
      const localFiles = this.vault.getMarkdownFiles();
      const toPush = localFiles.filter((f) => {
        const p = f.path.trim().replace(/\\/g, "/");
        return !remotePaths.has(p) && !deletedSet.has(p);
      });
      let pushed = 0;
      if (toPush.length) {
        try {
          await this.pushAllNow(toPush);
          pushed = toPush.length;
        } catch (e) {
          console.error("[Flux] push local-only:", e);
          new Notice(`Flux: push failed — ${errorMessage(e)}`);
        }
      }
      if (applied || removed || pushed) {
        const parts = [];
        if (applied) parts.push(`${applied} updated`);
        if (removed) parts.push(`${removed} deleted`);
        if (pushed) parts.push(`${pushed} pushed`);
        new Notice(`Flux: synced — ${parts.join(", ")}`);
      }
    } catch (e) {
      if ((e as Error).name === "AbortError") return;
      console.error("[Flux] pull error:", e);
      new Notice(`Flux: pull failed — ${errorMessage(e)}`);
    }
  }

  cancel(): void {
    this.abortController?.abort();
    for (const t of this.pushDebounce.values()) clearTimeout(t);
    this.pushDebounce.clear();
  }

  handleCreate(file: TAbstractFile): void {
    if (file instanceof TFile) this.pushFile(file);
  }
  handleModify(file: TAbstractFile): void {
    if (file instanceof TFile) this.pushFile(file);
  }
  async handleRename(file: TAbstractFile, oldPath: string): Promise<void> {
    if (this.applyingPull || !this.settings.enabled || !this.baseUrl) return;
    const norm = (p: string) => p.trim().replace(/\\/g, "/");
    const deleted = [norm(oldPath)];
    const files: PushFile[] = [];
    if (file instanceof TFile) {
      try {
        const content = await this.vault.read(file);
        files.push({ path: norm(file.path), content, hash: contentHash(content) });
      } catch (e) {
        console.error("[Flux] rename read error:", e);
        new Notice(`Flux: rename failed — ${errorMessage(e)}`);
        return;
      }
    }
    try {
      const res = await this.api("/push", { method: "POST", body: JSON.stringify({ files, deleted }) });
      if (!res.ok) {
        const body = (await res.json().catch(() => ({}))) as { error?: string };
        throw new Error(`Push failed: ${res.status}${body?.error ? ` — ${body.error}` : ""}`);
      }
      new Notice(`Flux: synced rename → ${file.path}`);
    } catch (e) {
      console.error("[Flux] rename push error:", e);
      new Notice(`Flux: rename failed — ${errorMessage(e)}`);
    }
  }

  async handleDelete(file: TAbstractFile): Promise<void> {
    if (this.applyingPull || !this.settings.enabled || !this.baseUrl) return;
    try {
      const res = await this.api("/push", {
        method: "POST",
        body: JSON.stringify({ files: [], deleted: [file.path] }),
      });
      if (!res.ok) throw new Error(`Push failed: ${res.status}`);
      new Notice(`Flux: deleted ${file.path}`);
    } catch (e) {
      console.error("[Flux] delete push error:", e);
      new Notice(`Flux: delete failed — ${errorMessage(e)}`);
    }
  }
}
