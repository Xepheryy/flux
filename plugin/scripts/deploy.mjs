import { cpSync, mkdirSync, existsSync } from "fs";
import { resolve, dirname } from "path";
import { fileURLToPath } from "url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const pluginDir = resolve(__dirname, "..");

const vaultPath = process.env.VAULT_PATH || resolve(pluginDir, "../test-vault");
const targetDir = resolve(vaultPath, ".obsidian", "plugins", "flux-sync");

if (!existsSync(targetDir)) {
  mkdirSync(targetDir, { recursive: true });
}

cpSync(resolve(pluginDir, "main.js"), resolve(targetDir, "main.js"));
cpSync(resolve(pluginDir, "manifest.json"), resolve(targetDir, "manifest.json"));

console.log("Deployed to:", targetDir);
