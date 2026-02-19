import type { App } from "obsidian";
import { Modal } from "obsidian";

const WARNING = `Flux will become the only source of truth for your vault.

After successful setup, all changes should go through Flux. Disable other sync solutions (e.g. iCloud, Dropbox, Obsidian Sync) for this vault.

By enabling Flux, you acknowledge that Flux will be the central source of truth.`;

class FluxWarningModal extends Modal {
  constructor(app: App, private onResult: (ok: boolean) => void) {
    super(app);
  }

  onOpen() {
    const { contentEl } = this;
    contentEl.empty();
    contentEl.createEl("h2", { text: "Flux - Source of Truth" });
    contentEl.createEl("p", { text: WARNING });
    const btns = contentEl.createDiv({ cls: "flux-modal-buttons" });
    btns.createEl("button", { text: "Cancel" }).onclick = () => { this.onResult(false); this.close(); };
    const ok = btns.createEl("button", { text: "I understand, enable Flux" });
    ok.addClass("mod-cta");
    ok.onclick = () => { this.onResult(true); this.close(); };
  }

  onClose() {
    this.onResult(false);
  }
}

export function showInstallWarning(app: App): Promise<boolean> {
  return new Promise((resolve) => {
    let done = false;
    const once = (v: boolean) => {
      if (!done) {
        done = true;
        resolve(v);
      }
    };
    new FluxWarningModal(app, once).open();
  });
}
