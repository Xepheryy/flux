# Flux

Sync your entire Obsidian vault to a Flux server (and optionally to GitHub). Flux is the central source of truth; disable other sync solutions for the vault.

## Developer quickstart

**Server**

```bash
cd server
cp .env.example .env   # set FLUX_GIT_OWNER, FLUX_GIT_REPO, FLUX_GIT_TOKEN
go run ./cmd/server     # or: air (hot reload)
```

Server listens on `:8080` (or `PORT`). It will exit at startup if the three env vars are unset.

**Plugin**

```bash
cd plugin
pnpm install
pnpm run build
```

Then copy `main.js` and `manifest.json` into your vault’s `.obsidian/plugins/flux-sync/`, or symlink the `plugin` folder there and enable the plugin. Point the plugin at `http://localhost:8080` (or your server URL).

**Test server:** `curl http://localhost:8080/health` → `ok`

---

## Plugin (Obsidian)

- Set **Endpoint URL** (e.g. `https://flux.example.com`) and optional Basic Auth.
- Enable sync to sync the **whole vault** (all markdown files). Pull runs first, then push on save. Renames and deletes are synced.

**Install from source:** copy `plugin/main.js`, `plugin/manifest.json`, and (if present) `plugin/styles.css` into your vault’s `.obsidian/plugins/flux-sync/` folder.

**Build:** `cd plugin && pnpm install && pnpm run build`

## Server (Go)

Runs the sync API; syncs to Git when `FLUX_GIT_OWNER`, `FLUX_GIT_REPO`, and `FLUX_GIT_TOKEN` are set (e.g. in `server/.env`). Fails at startup if any are missing.

```bash
cd server && go build -o flux-server ./cmd/server && ./flux-server
```

Hot reload: `cd server && air`

## Monorepo layout

- `plugin/` — Obsidian plugin (TypeScript)
- `server/` — Go API + GitHub sync
- Root `manifest.json` and this `README.md` are for the [Obsidian community plugin](https://github.com/obsidianmd/obsidian-releases) listing; keep them in sync with `plugin/manifest.json` when releasing.

## Releasing the plugin

We use [Semantic Versioning](https://semver.org/) (MAJOR.MINOR.PATCH): **MAJOR** for breaking changes, **MINOR** for new features (backward compatible), **PATCH** for bug fixes.

1. Bump version in `plugin/manifest.json` and root `manifest.json` (and `plugin/package.json`).
2. `cd plugin && pnpm run build`.
3. Create a GitHub release with tag = version (e.g. `1.0.0`), attach `manifest.json`, `main.js`, and `styles.css` from `plugin/`.
