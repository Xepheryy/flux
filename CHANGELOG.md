# Changelog

## 0.2.1

- **Backend:** Retry GitHub delete on 409 (SHA mismatch when moving/renaming files).
- **Plugin:** Rename handler path normalization and clearer push error message.

## 0.2.0

- **Whole vault sync:** Removed Flux folder setting; the plugin now syncs all markdown files in the vault. The server continues to load the full repo from GitHub on startup.

## 0.1.0

- Initial release. Sync vault (or a single folder) to Flux server and optional GitHub.
