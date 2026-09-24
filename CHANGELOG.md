# DCS CLI changes

## 3.9.0 (2026-09-24)

### Fixes

- Commands no longer pause for several seconds before printing. Every run used
  to ask the terminal what color its background is, even with `--no-color`,
  `NO_COLOR` or an explicit `--theme`, and a terminal that never answers left
  you waiting out the timeout. Those three now print straight away. Only
  automatic theme detection still asks.

## 3.8.3 (2026-09-22)

### Fixes

- `dcs status` follows `--theme`. It used to match the terminal's background
  whatever you passed.
- `dcs status` and `dcs ui` no longer ask the terminal for its background
  color a second time, which could add several seconds to each run in
  terminals that do not answer.

## 3.8.2 (2026-09-21)

### Workspace refinements

- Reference areas behave the same. Databases and Storage now list every
  subcommand at the first level, the way Operations always did, with the
  parent command's own help first.
- Choosing a server for sites reads as a picker. Its title and description say
  what you are choosing, and each server shows only provider, region and IPv4.
- Long values under a label wrap with a hanging indent, so continuation lines
  sit under the value column instead of returning to the left edge.

## 3.8.1 (2026-09-20)

### One voice in the terminal

Plain commands and the interactive workspace now share one color system,
resolved once per run from your terminal's background or `--theme`. The old
red heading and warm-grey table text are gone; every surface uses the same
accent and the same three greys, and body text always keeps your terminal's
own foreground, so output is readable on light terminals too.

`dcs status` prints the same overview you see in `dcs ui`: your account, then
the six areas with live server, site and database counts. The boxed dashboard
is retired. JSON output is unchanged.

### Details you can feel

- Every status shows a glyph and the word, `● active`, `◐ deploying`,
  `○ off`, `✕ failed`, so `--no-color`, pipes and colorblind readers lose
  nothing.
- Key-value output right-aligns labels and never prints a bare label with an
  empty value. When your account has no display name, the email stands in.
- Tables keep fixed-shape columns like status, region and IPv4 intact and only
  shrink free text, so sizes no longer truncate at 100 columns.
- `dcs --help` groups commands by area: Servers, Sites & deploys, Databases,
  Storage, Access, Operations.
- The workspace waits 150ms before showing a loading indicator, so fast reads
  never flash.

## 3.8.0 (2026-09-19)

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
