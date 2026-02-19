import type { TAbstractFile } from "obsidian";
import { Notice, TFile, Vault } from "obsidian";
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
  return err.cause instanceof Error ? err.cause.message : err.message;
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
    return !folder || path === folder || path.startsWith(folder + "/");
  }

  private async api(path: string, opts: RequestInit = {}): Promise<Response> {
    if (!this.baseUrl) throw new Error("Flux endpoint not configured");
    const headers: Record<string, string> = { "Content-Type": "application/json", ...(opts.headers as object) };
    const { username, password } = this.settings;
    if (username || password) {
      headers.Authorization = `Basic ${btoa(`${username}:${password}`)}`;
    }
    return fetch(this.baseUrl + path, { ...opts, headers });
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

  async pull(): Promise<void> {
    if (!this.settings.enabled || !this.baseUrl) return;
    this.abortController?.abort();
    this.abortController = new AbortController();
    try {
      const res = await this.api("/pull", { method: "GET", signal: this.abortController.signal });
      if (!res.ok) throw new Error(`Pull failed: ${res.status}`);
      const data: PullResponse = await res.json();
      this.applyingPull = true;
      let applied = 0;
      let removed = 0;
      try {
        for (const p of data.deleted || []) {
          if (!this.inScope(p)) continue;
          const f = this.vault.getAbstractFileByPath(p);
          if (f) {
            await this.vault.delete(f);
            removed++;
          }
        }
        for (const f of data.files || []) {
          if (!this.inScope(f.path)) continue;
          const existing = this.vault.getAbstractFileByPath(f.path);
          if (existing && existing instanceof TFile) {
            const cur = await this.vault.read(existing);
            if (contentHash(cur) !== f.hash) {
              await this.vault.modify(existing, f.content, { origin: ORIGIN_FLUX });
              applied++;
            }
          } else {
            await this.vault.create(f.path, f.content, { origin: ORIGIN_FLUX });
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
