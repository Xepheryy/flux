import type { App } from "obsidian";
import { PluginSettingTab, Setting } from "obsidian";
import type FluxPlugin from "./main";

export interface FluxSettings {
  endpoint: string;
  username: string;
  password: string;
  syncIntervalSeconds: number;
  enabled: boolean;
  acknowledgedWarning: boolean;
}

export const DEFAULT_SETTINGS: FluxSettings = {
  endpoint: "",
  username: "",
  password: "",
  syncIntervalSeconds: 30,
  enabled: false,
  acknowledgedWarning: false,
};

export class FluxSettingTab extends PluginSettingTab {
  constructor(app: App, public plugin: FluxPlugin) {
    super(app, plugin);
  }

  display() {
    const { containerEl, plugin } = this;
    containerEl.empty();
    const save = (updates: Partial<FluxSettings>) => {
      Object.assign(plugin.settings, updates);
      plugin.saveSettings();
    };

    new Setting(containerEl).setName("Endpoint URL").setDesc("Flux server URL (e.g. https://flux.shaun.zip)")
      .addText((t) => t.setPlaceholder("https://flux.example.com").setValue(plugin.settings.endpoint).onChange((v) => save({ endpoint: v.trim() })));

    new Setting(containerEl).setName("Username").setDesc("Basic Auth username")
      .addText((t) => t.setPlaceholder("username").setValue(plugin.settings.username).onChange((v) => save({ username: v })));

    new Setting(containerEl).setName("Password").setDesc("Basic Auth password")
      .addText((t) => t.setPlaceholder("password").setValue(plugin.settings.password).onChange((v) => save({ password: v })));

    new Setting(containerEl).setName("Pull interval (seconds)").setDesc("How often to pull (default 30)")
      .addText((t) => t.setPlaceholder("30").setValue(String(plugin.settings.syncIntervalSeconds)).onChange((v) => {
        const n = Number.parseInt(v, 10);
        if (!Number.isNaN(n) && n > 0) {
          save({ syncIntervalSeconds: n });
          plugin.restartPullInterval();
        }
      }));

    new Setting(containerEl).setName("Enable sync").setDesc("Flux will become the only source of truth.")
      .addToggle((t) => t.setValue(plugin.settings.enabled).onChange(async (v) => {
        if (v && !plugin.settings.acknowledgedWarning) {
          const { showInstallWarning } = await import("./install-warning");
          if (!(await showInstallWarning(this.app))) return t.setValue(false);
          save({ acknowledgedWarning: true });
        }
        save({ enabled: v });
        plugin.setSyncEnabled(v);
      }));
  }
}
