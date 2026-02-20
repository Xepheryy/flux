import { Notice, Plugin } from "obsidian";
import { DEFAULT_SETTINGS, FluxSettings, FluxSettingTab } from "./settings";
import { FluxSync } from "./sync";

export default class FluxPlugin extends Plugin {
  settings: FluxSettings;
  private sync: FluxSync | null = null;
  private pullInterval: ReturnType<typeof setInterval> | null = null;

  async onload() {
    await this.loadSettings();
    this.addCommand({ id: "flux-sync", name: "Sync", callback: () => this.syncNow() });
    this.addCommand({ id: "flux-pull", name: "Pull changes", callback: () => this.pull() });
    this.addCommand({ id: "flux-push", name: "Push all changes", callback: () => this.pushAll() });
    this.addSettingTab(new FluxSettingTab(this.app, this));
    this.sync = new FluxSync(this.settings, this.app.vault);
    if (this.settings.enabled) this.startSync();
  }

  onunload() {
    this.sync?.cancel();
    this.clearPullInterval();
  }

  async loadSettings() {
    this.settings = { ...DEFAULT_SETTINGS, ...(await this.loadData()) };
  }

  async saveSettings() {
    await this.saveData(this.settings);
  }

  setSyncEnabled(enabled: boolean) {
    enabled ? this.startSync() : this.stopSync();
  }

  restartPullInterval() {
    this.clearPullInterval();
    if (this.settings.enabled && this.settings.syncIntervalSeconds > 0) {
      this.pullInterval = setInterval(() => this.pull(), this.settings.syncIntervalSeconds * 1000);
    }
  }

  private async startSync() {
    if (!this.sync) return;
    this.sync.updateSettings(this.settings);
    const folder = (this.settings.fluxFolder || "").trim().replace(/^\/|\/$/g, "");
    if (folder && !this.app.vault.getAbstractFileByPath(folder)) await this.app.vault.createFolder(folder);
    // Pull first so we get missing notes from the server before pushing local state
    await this.pull();
    // Then push existing in-scope files so server has our current state
    const files = this.app.vault.getMarkdownFiles().filter((f) => this.sync!.inScope(f.path));
    if (files.length) {
      try {
        await this.sync.pushAllNow(files);
      } catch (e) {
        console.error("[Flux] initial push:", e);
        new Notice(`Flux: initial push failed — ${e instanceof Error ? e.message : String(e)}`);
      }
    }
    this.registerEvent(this.app.vault.on("create", (f) => this.sync!.handleCreate(f)));
    this.registerEvent(this.app.vault.on("modify", (f) => this.sync!.handleModify(f)));
    this.registerEvent(this.app.vault.on("rename", (f, old) => this.sync!.handleRename(f, old)));
    this.registerEvent(this.app.vault.on("delete", (f) => this.sync!.handleDelete(f)));
    this.restartPullInterval();
  }

  private stopSync() {
    this.sync?.cancel();
    this.clearPullInterval();
  }

  private clearPullInterval() {
    if (this.pullInterval) {
      clearInterval(this.pullInterval);
      this.pullInterval = null;
    }
  }

  private async syncNow() {
    await this.pull();
    await this.pushAll();
  }

  private pull() {
    return this.sync?.pull();
  }

  private async pushAll() {
    if (!this.sync) return;
    const files = this.app.vault.getMarkdownFiles().filter((f) => this.sync!.inScope(f.path));
    await this.sync.pushFiles(files);
    new Notice("Flux: pushed all files");
  }
}
