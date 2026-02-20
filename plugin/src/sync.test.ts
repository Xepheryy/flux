import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { requestUrl } from "obsidian";
import { FluxSync } from "./sync";
import type { FluxSettings } from "./settings";

const defaultSettings: FluxSettings = {
  endpoint: "https://flux.test",
  username: "u",
  password: "p",
  syncIntervalSeconds: 30,
  fluxFolder: "Flux",
  enabled: true,
  acknowledgedWarning: true,
};

describe("FluxSync", () => {
  let vault: {
    create: ReturnType<typeof vi.fn>;
    createFolder: ReturnType<typeof vi.fn>;
    getAbstractFileByPath: ReturnType<typeof vi.fn>;
    getMarkdownFiles: ReturnType<typeof vi.fn>;
    read: ReturnType<typeof vi.fn>;
    modify: ReturnType<typeof vi.fn>;
    delete: ReturnType<typeof vi.fn>;
  };

  beforeEach(() => {
    vi.mocked(requestUrl).mockReset();

    vault = {
      create: vi.fn().mockResolvedValue(undefined),
      createFolder: vi.fn().mockResolvedValue(undefined),
      getAbstractFileByPath: vi.fn().mockReturnValue(null),
      getMarkdownFiles: vi.fn().mockReturnValue([]),
      read: vi.fn(),
      modify: vi.fn().mockResolvedValue(undefined),
      delete: vi.fn().mockResolvedValue(undefined),
    };
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  describe("pull", () => {
    it("creates missing file and parent folder when server returns new file", async () => {
      requestUrl.mockResolvedValue({
        status: 200,
        json: Promise.resolve({
          files: [{ path: "Flux/note.md", content: "# Hi", hash: "sha256:abc123" }],
          deleted: [],
        }),
      });

      vault.getAbstractFileByPath.mockImplementation((path: string) => {
        if (path === "Flux" || path === "Flux/note.md") return null;
        return null;
      });

      const sync = new FluxSync(defaultSettings, vault as any);
      await sync.pull();

      expect(vault.createFolder).toHaveBeenCalledWith("Flux");
      expect(vault.create).toHaveBeenCalledWith("Flux/note.md", "# Hi", {});
      expect(vault.create).toHaveBeenCalledTimes(1);
    });

    it("does not create folder when parent already exists", async () => {
      requestUrl.mockResolvedValue({
        status: 200,
        json: Promise.resolve({
          files: [{ path: "Flux/note.md", content: "# Hi", hash: "sha256:abc" }],
          deleted: [],
        }),
      });

      vault.getAbstractFileByPath.mockImplementation((path: string) => {
        if (path === "Flux") return {}; // folder exists
        return null;
      });

      const sync = new FluxSync(defaultSettings, vault as any);
      await sync.pull();

      expect(vault.createFolder).not.toHaveBeenCalled();
      expect(vault.create).toHaveBeenCalledWith("Flux/note.md", "# Hi", {});
    });

    it("updates existing file when hash differs", async () => {
      const { TFile } = await import("obsidian");
      const existing = new TFile("Flux/note.md");

      requestUrl.mockResolvedValue({
        status: 200,
        json: Promise.resolve({
          files: [{ path: "Flux/note.md", content: "# Updated", hash: "sha256:new" }],
          deleted: [],
        }),
      });

      vault.getAbstractFileByPath.mockReturnValue(existing);
      vault.read.mockResolvedValue("# Old");

      const sync = new FluxSync(defaultSettings, vault as any);
      await sync.pull();

      expect(vault.read).toHaveBeenCalledWith(existing);
      expect(vault.modify).toHaveBeenCalledWith(existing, "# Updated", {});
      expect(vault.create).not.toHaveBeenCalled();
    });

    it("deletes local file when server reports deleted", async () => {
      const { TFile } = await import("obsidian");
      const existing = new TFile("Flux/gone.md");

      requestUrl.mockResolvedValue({
        status: 200,
        json: Promise.resolve({ files: [], deleted: ["Flux/gone.md"] }),
      });

      vault.getAbstractFileByPath.mockReturnValue(existing);

      const sync = new FluxSync(defaultSettings, vault as any);
      await sync.pull();

      expect(vault.delete).toHaveBeenCalledWith(existing);
    });

    it("skips file outside fluxFolder scope", async () => {
      requestUrl.mockResolvedValue({
        status: 200,
        json: Promise.resolve({
          files: [{ path: "Other/note.md", content: "# No", hash: "x" }],
          deleted: [],
        }),
      });

      const sync = new FluxSync(defaultSettings, vault as any);
      await sync.pull();

      expect(vault.create).not.toHaveBeenCalled();
      expect(vault.createFolder).not.toHaveBeenCalled();
    });
  });

  describe("inScope", () => {
    it("returns true for path in fluxFolder", () => {
      const sync = new FluxSync(defaultSettings, vault as any);
      expect(sync.inScope("Flux")).toBe(true);
      expect(sync.inScope("Flux/note.md")).toBe(true);
    });

    it("returns false for path outside fluxFolder", () => {
      const sync = new FluxSync(defaultSettings, vault as any);
      expect(sync.inScope("Other/note.md")).toBe(false);
    });
  });
});
