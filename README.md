# Flux

Sync your Obsidian vault to a Flux server (and optionally to GitHub). Flux is the central source of truth; disable other sync solutions for the vault.

## Plugin (Obsidian)

- Configure **Flux folder** (default: `Flux`) so only that folder syncs; the rest of the vault is untouched.
- Set **Endpoint URL** (e.g. `https://flux.example.com`) and optional Basic Auth.
- Enable sync; pull runs first, then push on save. Renames and deletes are synced.

**Install from source:** copy `plugin/main.js`, `plugin/manifest.json`, and (if present) `plugin/styles.css` into your vault’s `.obsidian/plugins/flux-sync/` folder.

**Build:** `cd plugin && pnpm install && pnpm run build`

## Server (Go)

Runs the sync API; can sync to GitHub when `FLUX_GITHUB_OWNER`, `FLUX_GITHUB_REPO`, and `FLUX_GITHUB_TOKEN` are set (e.g. in `server/.env`).

```bash
cd server && go build -o flux-server ./cmd/server && ./flux-server
```

Hot reload: `cd server && air`

## Monorepo layout

- `plugin/` — Obsidian plugin (TypeScript)
- `server/` — Go API + GitHub sync
- Root `manifest.json` and this `README.md` are for the [Obsidian community plugin](https://github.com/obsidianmd/obsidian-releases) listing; keep them in sync with `plugin/manifest.json` when releasing.

## Releasing the plugin

1. Bump version in `plugin/manifest.json` (and root `manifest.json`).
2. `cd plugin && pnpm run build`.
3. Create a GitHub release with tag = version (e.g. `1.0.0`), attach `manifest.json`, `main.js`, and `styles.css` from `plugin/`.
