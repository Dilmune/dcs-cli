# DCS CLI

The command-line client for [Dilmune Cloud](https://dilmune.com).
Manage cloud resources and deployments using your Dilmune Cloud account.

This repository contains the CLI client, not the Dilmune Cloud backend.

## Install

Install with one command on Linux or macOS:

```sh
curl -fsSL https://raw.githubusercontent.com/Dilmune/dcs-cli/main/install.sh | sh
```

The script downloads the release archive for your OS and architecture, verifies
its SHA-256 against the release's `checksums.txt`, and puts `dcs` in
`~/.local/bin`. Set `DCS_VERSION=vX.Y.Z` to install a specific release and
`DCS_INSTALL_DIR=/some/dir` to install somewhere else. It refuses to run as
root unless `DCS_INSTALL_DIR` is set, and tells you the PATH line to add if the
directory is not already on it.

With Homebrew:

```sh
brew install dilmune/tap/dcs
```

Homebrew shows the tap as `Untrusted` in `brew tap-info dilmune/tap`. That is
expected for any tap outside homebrew-core and does not block the install. To
mark it trusted:

```sh
brew trust --tap dilmune/tap
```

For manual installation, download the archive for your operating system and
architecture from the [latest release](https://github.com/Dilmune/dcs-cli/releases/latest).
Verify it against the release's `checksums.txt` before extracting it.

To upgrade an existing Homebrew installation:

```sh
brew update
brew upgrade dilmune/tap/dcs
```

## Get started

Create a DCS API key in the [Dilmune Cloud portal](https://cloud.dilmune.com/settings/api-keys),
then authenticate:

```sh
dcs login
dcs whoami
dcs status
```

The CLI uses a DCS API key, not S3 access credentials. Keep keys out of source
control, screenshots and public issue reports. For automation, provide
`DCS_API_KEY` through your CI system's secret storage.

Discover commands and their options with:

```sh
dcs --help
dcs servers --help
dcs sites --help
dcs storage --help
```

## What you can manage

| Area | Commands |
| --- | --- |
| Servers | `dcs servers`, `dcs ssh`, `dcs firewall`, `dcs software` |
| Sites and deployments | `dcs sites`, `dcs init`, `dcs deploy`, `dcs env`, `dcs logs` |
| Databases | `dcs db` |
| Files and buckets | `dcs storage` |
| Access | `dcs keys`, `dcs api-keys` |
| Background work | `dcs cron`, `dcs daemons` |

Start with read-only commands for resources you already own:

```sh
dcs servers list
dcs sites list --server '<server-id>'
dcs db list --server '<server-id>'
dcs storage buckets
dcs storage ls --bucket '<exact-bucket-name-or-full-id>'
```

Use `--json` with structured-output commands, such as `dcs servers list --json`
or `dcs status --json`. Interactive workflows and some mutation commands still
use human-readable output; check the command you plan to use before scripting it.
Account permissions and the service's normal usage charges still apply when
managing cloud resources.

Server, site, database, SSH-key and API-key deletion accept `--json` (or
`--output json`) with `--force`. They return `{"id":"<resolved-id>","success":true}`
when the API accepts the deletion request; background cleanup may still be
running. Without `--force`, JSON deletion fails without deleting anything.
Failed requests return a nonzero exit code and no success object on stdout.

## Interactive workspace

```sh
dcs ui
```

An optional, read-only terminal workspace for the whole CLI. Browse live servers
and their sites; explore command references for databases, storage, access, and
operations. References come from the CLI itself and **never execute**. Deployments,
SSH sessions, and resource changes still use the ordinary commands.

Use the arrow keys and Enter to explore, `/` to find a command, Esc to go back,
`r` to refresh a live view, and `?` for help. Press `q` or Ctrl+C to exit. The
workspace restores your terminal when it closes. It requires at least 48 columns
and 20 rows; wider terminals show an additional detail pane.

The workspace opens directly into the menu. A compact Dilmune mark appears on
first use and adapts to the terminal size. Opening an item remembers the
introduction; `dcs ui --welcome` shows it again. This preference is stored
separately from your credentials.

Terminal accents follow the background automatically. Override them with
`--theme light`, `--theme dim`, or `--theme dark`; `--no-color` and `NO_COLOR` disable
color. Your terminal's background remains unchanged.

Both input and output must be terminals. Do not combine `ui` with `--json`,
`--output`, `--quiet`, or `--debug`. Use the existing commands for scripts and
structured output; they do not show the new welcome or save its preference.

## Shell completion

Generate a completion script for your shell:

```sh
dcs completion bash
dcs completion zsh
dcs completion fish
dcs completion powershell
```

Run `dcs completion <shell> --help` for installation instructions. Generating
completions does not require login.

See [CHANGELOG.md](CHANGELOG.md) for release changes.

## Build and test

Use the Go toolchain specified by `toolchain` in `go.mod`. Run these commands
from the CLI source directory:

```sh
GOWORK=off go build -mod=readonly -trimpath -o ./bin/dcs ./cmd/dcs
./bin/dcs version --json
GOWORK=off go mod tidy -diff
GOWORK=off go mod verify
GOWORK=off go vet ./...
GOWORK=off go test -race -count=1 ./...
```

Tests use local fixtures and do not require a Dilmune Cloud account. Do not
provide a production API key when running them.

Generate command documentation locally:

```sh
./bin/dcs docs markdown --dir ./generated-docs
```

Maintainers: see [RELEASING.md](RELEASING.md) for checked, source-backed builds
and the separate publishing gate.

## License and scope

The DCS CLI source and documentation in this repository are licensed under
the [MIT License](LICENSE), copyright DILMUNE CLOUD W.L.L. This license does
not cover the Dilmune Cloud backend or other private DCS software.

See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) for dependency notices.

Dilmune names and logos identify the official product and service. The software
license does not grant permission to imply endorsement or affiliation.
