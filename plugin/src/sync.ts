import type { TAbstractFile } from "obsidian";
import { Notice, requestUrl, TFile, Vault } from "obsidian";
import type { FluxSettings } from "./settings";

const ORIGIN_FLUX = "flux";
const PUSH_DEBOUNCE_MS = 500;

function contentHash(str: string): string {
  let h = 0;
  for (let i = 0; i < str.length; i++) h = ((h << 5) - h + str.charCodeAt(i)) | 0;
  return `sha256:${Math.abs(h).toString(36)}${str.length}`;
}

function normalizeFolder(s: string): string {
  return (s || "").trim().replace(/^\/|\/$/g, "");
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

  inScope(path: string): boolean {
    const folder = normalizeFolder(this.settings.fluxFolder);
    return !folder || path === folder || path.startsWith(`${folder}/`);
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
    const json = (): Promise<unknown> => {
      const j = res.json;
      if (j != null && typeof (j as Promise<unknown>).then === "function") return j as Promise<unknown>;
      if (typeof j === "string") return Promise.resolve(JSON.parse(j));
      return Promise.resolve(j);
    };
    return {
      ok: res.status >= 200 && res.status < 300,
      status: res.status,
      json,
    };
  }

  async pushFile(file: TFile): Promise<void> {
    if (this.applyingPull || !this.settings.enabled || !this.baseUrl || file.extension !== "md" || !this.inScope(file.path)) return;

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
      if (f.extension !== "md" || !this.inScope(f.path)) continue;
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
      try {
        for (const p of data.deleted || []) {
          if (!this.inScope(p)) continue;
          const f = this.vault.getAbstractFileByPath(p);
          if (f) {
            await this.vault.delete(f);
            removed++;
          }
        }
        for (const f of filesList) {
          if (!f || typeof f.path !== "string" || typeof f.content !== "string") continue;
          if (!this.inScope(f.path)) continue;
          const existing = this.vault.getAbstractFileByPath(f.path);
          if (existing && existing instanceof TFile) {
            const cur = await this.vault.read(existing);
            if (contentHash(cur) !== f.hash) {
              await this.vault.modify(existing, f.content, {});
              applied++;
            }
          } else {
            const dir = f.path.includes("/") ? f.path.replace(/\/[^/]+$/, "") : "";
            if (dir && !this.vault.getAbstractFileByPath(dir)) await this.vault.createFolder(dir);
            await this.vault.create(f.path, f.content, {});
            applied++;
          }
        }
      } finally {
        this.applyingPull = false;
      }
      new Notice(applied || removed ? `Flux: pulled ${applied} updated, ${removed} deleted` : "Flux: pull complete (no changes)");
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
    const dropOld = this.inScope(oldPath);
    const pushNew = file instanceof TFile && this.inScope(file.path);
    if (!dropOld && !pushNew) return;
    const deleted = dropOld ? [oldPath] : [];
    const files: PushFile[] = [];
    if (pushNew) {
      const content = await this.vault.read(file);
      files.push({ path: file.path, content, hash: contentHash(content) });
    }
    try {
      const res = await this.api("/push", {
        method: "POST",
        body: JSON.stringify({ files, deleted }),
      });
      if (!res.ok) throw new Error(`Push failed: ${res.status}`);
      new Notice(`Flux: synced rename → ${file.path}`);
    } catch (e) {
      console.error("[Flux] rename push error:", e);
      new Notice(`Flux: rename failed — ${errorMessage(e)}`);
    }
  }

  async handleDelete(file: TAbstractFile): Promise<void> {
    if (this.applyingPull || !this.settings.enabled || !this.baseUrl || !this.inScope(file.path)) return;
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
