# CLI builds and releases

The public build uses only this repository. A release tag must be an existing
stable vX.Y.Z tag on the checked-out commit, and X.Y.Z must equal the Version
declaration in internal/client/client.go. Never move a published tag or replace
an existing release. Use a new version for changes.

Update CHANGELOG.md from verified changes before preparing the tag. Replace the
unreleased marker with the release date only when the release is approved.

## Checks before publishing

The CLI checks workflow runs race-enabled tests on GitHub-hosted Linux, macOS
and Windows runners. A separate read-only job scans source for all six supported
OS/architecture combinations, scans source/history for secrets, and builds with
the exact Go toolchain in go.mod.

GoReleaser 2.18.1 packages Linux/macOS/Windows on AMD64 and ARM64. Its binary
vulnerability hook runs for every target. Each archive must contain exactly the
binary, LICENSE, README.md and THIRD_PARTY_NOTICES.md. The archive checker verifies
all six checksums, document bytes, embedded Go version, platform, source revision
and clean-tree build status. It also runs the native binary's version command.

Build dates and archive timestamps derive from the source commit, not wall-clock
time. Refresh the dependency notices whenever dependencies or the Go toolchain
change. Node 24.21.0 is used only for build-verification scripts, not by the CLI.
Third-party actions are pinned to commit SHAs; review pin updates together with
their upstream release notes.

## Local verification

From a clean, standalone Git checkout with the pinned Go toolchain, Node, Bash,
jq, sha256sum, tar and unzip installed:

```sh
export GOWORK=off
export GOTOOLCHAIN=local
export GOFLAGS=-mod=readonly
go mod tidy -diff
go mod verify
go vet ./...
go test -race -count=1 ./...
node --test scripts/release-check.test.mjs
go install golang.org/x/vuln/cmd/govulncheck@v1.8.0
```

Install GoReleaser 2.18.1 and put govulncheck on PATH. With no publishing tokens
in the environment, use a fresh dist directory:

```sh
node scripts/release-check.mjs source vX.Y.Z
GORELEASER_CURRENT_TAG=vX.Y.Z goreleaser release --config .goreleaser-public.yaml --skip=publish
node scripts/release-check.mjs archives dist vX.Y.Z
```

This builds archives locally; it does not create a GitHub release. Untagged CI
uses snapshots, clearly marked as such. A snapshot skips GoReleaser's release
validation and is not evidence that a real release tag passed the checks above.
The public GoReleaser configuration has publishing disabled and contains no
Homebrew publisher.

## Publication gate

The Release CLI workflow is manual. Select an existing tag, not a branch.
The default publish=false runs the full checks and produces a seven-file
workflow artifact without changing releases or the Homebrew tap.

Publishing additionally requires all of the following:

- The official Dilmune/dcs-cli repository.
- publish=true supplied explicitly.
- Repository variable CLI_PUBLIC_RELEASE_ENABLED set to true.
- The cli-release environment, with required reviewers and tag-only deployment
  restrictions configured by a maintainer before enabling publication.
- Every native test and packaging check passing in the same workflow run.

Each build attempt keeps its own immutable artifact. The write-permission job
requires the single artifact ID returned by its successful checks, then downloads
only that artifact from the same run; retries do not overwrite earlier evidence.
It does not check out source, execute artifact contents or rebuild anything.
It rechecks the exact seven filenames and checksums, resolves the remote tag to
the event commit, then checks for an existing release or draft before creating
a draft using --verify-tag. Uploads finish before a separate operation publishes
the draft. It never deletes or overwrites a release to recover from an error.
If a request fails, inspect the remote draft/release and assets before deciding
what to do next; a failed response does not prove that publication failed.

Before activation, verify repository/environment permissions, protect release
tags against deletion/movement, and retire any other workflow that can publish
different binaries under these tags. Merely naming an environment does not
configure its approval rules.

Homebrew tap updates are separate and must use the verified published archive
checksums. This workflow does not receive a cross-repository tap token. Changing
the publisher or install channel requires its own reviewed cutover.
