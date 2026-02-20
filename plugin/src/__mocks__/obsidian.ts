import { vi } from "vitest";

export class TFile {
  path: string;
  constructor(path: string) {
    this.path = path;
  }
}

export const requestUrl = vi.fn();
export const Notice = vi.fn();
export class Vault {}
