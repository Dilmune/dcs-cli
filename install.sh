#!/bin/sh
# Installs the DCS CLI from a GitHub release of Dilmune/dcs-cli.
#
#   curl -fsSL https://raw.githubusercontent.com/Dilmune/dcs-cli/main/install.sh | sh
#
# Knobs:
#   DCS_VERSION      release tag to install, for example v3.8.1 (default: latest)
#   DCS_INSTALL_DIR  directory for the dcs binary (default: $HOME/.local/bin)
#
# Root is refused unless DCS_INSTALL_DIR is set, so a piped run cannot write
# into a system directory by accident.

set -eu

REPO="Dilmune/dcs-cli"
API_URL="https://api.github.com/repos/${REPO}/releases/latest"
LATEST_URL="https://github.com/${REPO}/releases/latest"
DOWNLOAD_BASE="https://github.com/${REPO}/releases/download"

tmp_dir=""

cleanup() {
  if [ -n "$tmp_dir" ]; then
    rm -rf "$tmp_dir"
  fi
}
trap cleanup EXIT

fail() {
  echo "dcs install: $1" >&2
  exit 1
}

has() {
  command -v "$1" >/dev/null 2>&1
}

detect_os() {
  case "$(uname -s)" in
    Linux) echo linux ;;
    Darwin) echo darwin ;;
    *) fail "unsupported operating system: $(uname -s) (Linux and macOS only)" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64 | amd64) echo amd64 ;;
    aarch64 | arm64) echo arm64 ;;
    *) fail "unsupported architecture: $(uname -m) (amd64 and arm64 only)" ;;
  esac
}

# fetch URL DEST: writes the response body to DEST, fails on any HTTP error.
fetch() {
  if has curl; then
    curl -fsSL --retry 2 -o "$2" "$1" || fail "download failed: $1"
  elif has wget; then
    wget -q -O "$2" "$1" || fail "download failed: $1"
  else
    fail "neither curl nor wget is installed"
  fi
}

# Stable tags only: vX.Y.Z with nothing else, so a header suffix such as
# wget's " [following]" can never leak into a download URL.
is_release_tag() {
  case "$1" in
    *[!v0-9.]*) return 1 ;;
    v[0-9]*.[0-9]*.[0-9]*) return 0 ;;
    *) return 1 ;;
  esac
}

# latest_tag_from_redirect: the release page redirects to /releases/tag/vX.Y.Z
# and is not rate limited, unlike the API. Prints nothing when it cannot tell.
latest_tag_from_redirect() {
  if has curl; then
    headers="$(curl -fsSLI "$LATEST_URL" 2>/dev/null)" || return 0
  elif has wget; then
    # wget writes server headers to stderr; --spider sends HEAD and follows redirects.
    headers="$(wget --spider -S "$LATEST_URL" 2>&1)" || return 0
  else
    return 0
  fi
  # wget appends " [following]" to the header line; the URL is always the second field.
  location="$(printf '%s\n' "$headers" | tr -d '\r' | grep -i '^ *location:' | tail -n 1 | awk '{print $2}')"
  case "$location" in
    */releases/tag/*) echo "${location##*/releases/tag/}" ;;
  esac
}

latest_tag_from_api() {
  fetch "$API_URL" "$tmp_dir/latest.json"
  sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' "$tmp_dir/latest.json" | head -n 1
}

resolve_version() {
  if [ -n "${DCS_VERSION:-}" ]; then
    is_release_tag "$DCS_VERSION" || fail "DCS_VERSION must look like vX.Y.Z, got: $DCS_VERSION"
    echo "$DCS_VERSION"
    return
  fi
  tag="$(latest_tag_from_redirect)"
  if ! is_release_tag "$tag"; then
    tag="$(latest_tag_from_api)"
  fi
  if ! is_release_tag "$tag"; then
    fail "could not resolve the latest release tag from $LATEST_URL or $API_URL (set DCS_VERSION=vX.Y.Z to pin one)"
  fi
  echo "$tag"
}

sha256_of() {
  if has sha256sum; then
    sha256sum "$1" | cut -d ' ' -f 1
  elif has shasum; then
    shasum -a 256 "$1" | cut -d ' ' -f 1
  else
    fail "neither sha256sum nor shasum is installed"
  fi
}

# verify_checksum ARCHIVE CHECKSUMS NAME: compares the archive's sha256 with
# the line for NAME in CHECKSUMS; a missing line fails the same as a mismatch.
verify_checksum() {
  expected="$(grep " $3\$" "$2" | cut -d ' ' -f 1)"
  if [ -z "$expected" ]; then
    fail "checksums.txt has no entry for $3"
  fi
  actual="$(sha256_of "$1")"
  if [ "$actual" != "$expected" ]; then
    fail "checksum mismatch for $3: expected $expected, got $actual"
  fi
}

path_hint() {
  case "$(basename "${SHELL:-sh}")" in
    zsh) echo "echo 'export PATH=\"$1:\$PATH\"' >> ~/.zshrc && source ~/.zshrc" ;;
    bash) echo "echo 'export PATH=\"$1:\$PATH\"' >> ~/.bashrc && source ~/.bashrc" ;;
    fish) echo "fish_add_path $1" ;;
    *) echo "export PATH=\"$1:\$PATH\"" ;;
  esac
}

main() {
  if [ "$(id -u)" -eq 0 ] && [ -z "${DCS_INSTALL_DIR+x}" ]; then
    fail "refusing to run as root; set DCS_INSTALL_DIR explicitly to install for root"
  fi
  has tar || fail "tar is not installed"
  has mktemp || fail "mktemp is not installed"

  install_dir="${DCS_INSTALL_DIR:-$HOME/.local/bin}"
  os="$(detect_os)"
  arch="$(detect_arch)"
  tmp_dir="$(mktemp -d)"

  version="$(resolve_version)"
  archive="dcs_${os}_${arch}.tar.gz"
  base="${DOWNLOAD_BASE}/${version}"

  echo "Installing dcs ${version} (${os}/${arch}) to ${install_dir}"
  fetch "${base}/${archive}" "$tmp_dir/$archive"
  fetch "${base}/checksums.txt" "$tmp_dir/checksums.txt"
  verify_checksum "$tmp_dir/$archive" "$tmp_dir/checksums.txt" "$archive"

  tar -xzf "$tmp_dir/$archive" -C "$tmp_dir" dcs || fail "could not extract $archive"
  [ -f "$tmp_dir/dcs" ] || fail "$archive does not contain a dcs binary"

  mkdir -p "$install_dir" || fail "could not create $install_dir"
  cp "$tmp_dir/dcs" "$install_dir/dcs.tmp.$$" || fail "could not write to $install_dir"
  chmod 755 "$install_dir/dcs.tmp.$$"
  mv "$install_dir/dcs.tmp.$$" "$install_dir/dcs" || fail "could not replace $install_dir/dcs"

  "$install_dir/dcs" version || fail "installed binary failed to run"

  case ":$PATH:" in
    *":$install_dir:"*) ;;
    *)
      echo "$install_dir is not on your PATH. Add it with:"
      echo "  $(path_hint "$install_dir")"
      ;;
  esac
}

main
