# DCS CLI changes

## 3.8.0 (unreleased)

### Interactive workspace

Run `dcs ui` to explore servers, sites, databases, storage, access and operations
from a terminal menu. Server and site views use your account's live data. Other
areas provide searchable command references; selecting a reference never runs it.
The workspace is read-only. Use ordinary commands for deployments and changes.

The menu adapts to your terminal and supports light, dim, dark and no-color modes.
The compact Dilmune mark appears on first use; `dcs ui --welcome` shows it again.
Existing scripts do not open the workspace or show its introduction.

### Security

The API client rejects redirects from HTTPS to HTTP, keeping credentials off an
unencrypted connection when an API endpoint redirects.

API keys created with `--expires 30d` or `--expires 90d` now receive the requested
expiry. Invalid durations fail before a key is created. Logout removes a saved
keychain credential instead of allowing the next command to reload it. An API
key supplied through `DCS_API_KEY` must still be unset in your shell.

This does not change existing keys. If you used `--expires` with an older CLI,
review those keys and their expiry dates in the portal.

### Command fixes

- Site and SSH-key creation send the field names required by the current API.
- Dashboard counts, site SSL and Git details, and database backup details display
  the current API values. Existing serialized CLI field names are preserved.
- Database creation accepts the existing `mysql` and `postgres` aliases and sends
  the supported `mysql8` and `postgres16` values.
- `deploy --json` emits JSON without the human-readable deployment preamble.
- Server, site, database, SSH-key and API-key deletion support `--json` and
  `--output json`. Use `--force` to confirm explicitly; success returns the
  resolved resource `id` and `success: true`, without prompts or progress output.
  Success means the API accepted the request, not that asynchronous cleanup has
  finished. Failed requests return a nonzero exit code and no success object.
- Environment commands read the API's `vars` response. Setting or removing a
  variable preserves the other values from that snapshot, and an unreadable
  snapshot stops the update. The API still replaces the full map, so avoid
  simultaneous environment edits from multiple clients.

### Source and builds

The CLI-only source is prepared for publication under MIT, with dependency
notices and checks for source-backed macOS, Linux and Windows builds on AMD64
and ARM64. The Dilmune Cloud backend is not part of this source release.

## 3.7.1

Updated the CLI build toolchain and dependencies. Available from the
[v3.7.1 release](https://github.com/Dilmune/dcs-cli/releases/tag/v3.7.1).

## 3.7.0

- Select named buckets using `dcs storage buckets` and `--bucket` on file commands.
  Existing commands without `--bucket` retain their default-storage behavior.
- Interrupted downloads leave the existing destination file intact.
- Login and `whoami` read the signed-in identity correctly.
- Update notices only recommend newer versions and stay out of structured output.
