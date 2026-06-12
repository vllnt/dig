#!/bin/sh
# dig installer — https://dig.vllnt.com/install.sh
#
#   curl -fsSL https://dig.vllnt.com/install.sh | sh
#
# Downloads the latest release binary for your OS/arch from GitHub, verifies it
# against the published checksums, and installs `dig` to a bin directory on your
# PATH (override with DIG_INSTALL_DIR). Pin a version with DIG_VERSION=vX.Y.Z.
#
# Local-first by design: this script only talks to GitHub to fetch the release.
set -eu

REPO="vllnt/dig"
BINARY="dig"
INSTALL_DIR="${DIG_INSTALL_DIR:-}"

err() { printf 'install: %s\n' "$1" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

# --- detect platform -------------------------------------------------------
os="$(uname -s)"
case "$os" in
  Linux) os="linux" ;;
  Darwin) os="darwin" ;;
  *) err "unsupported OS '$os' — use 'go install github.com/${REPO}/cmd/dig@latest' or grab a binary from https://github.com/${REPO}/releases" ;;
esac

arch="$(uname -m)"
case "$arch" in
  x86_64 | amd64) arch="amd64" ;;
  arm64 | aarch64) arch="arm64" ;;
  *) err "unsupported architecture '$arch'" ;;
esac

# --- pick a downloader -----------------------------------------------------
if have curl; then
  dl() { curl -fsSL "$1"; }
  dlo() { curl -fsSL "$1" -o "$2"; }
elif have wget; then
  dl() { wget -qO- "$1"; }
  dlo() { wget -qO "$2" "$1"; }
else
  err "need curl or wget"
fi

# --- resolve version -------------------------------------------------------
version="${DIG_VERSION:-}"
if [ -z "$version" ]; then
  version="$(dl "https://api.github.com/repos/${REPO}/releases/latest" \
    | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1)"
  [ -n "$version" ] || err "could not determine the latest release — no published release yet? Try 'go install github.com/${REPO}/cmd/dig@latest'"
fi
num="${version#v}"

# --- download + verify -----------------------------------------------------
archive="${BINARY}_${num}_${os}_${arch}.tar.gz"
base="https://github.com/${REPO}/releases/download/${version}"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

printf 'install: downloading %s %s (%s/%s)\n' "$BINARY" "$version" "$os" "$arch"
dlo "${base}/${archive}" "${tmp}/${archive}" || err "download failed for ${archive}"

if dlo "${base}/checksums.txt" "${tmp}/checksums.txt" 2>/dev/null; then
  ( cd "$tmp" && grep " ${archive}\$" checksums.txt | (
      if have sha256sum; then sha256sum -c -
      elif have shasum; then shasum -a 256 -c -
      else printf 'install: no sha256 tool, skipping checksum\n' >&2; fi
    ) ) || err "checksum verification failed"
fi

tar -xzf "${tmp}/${archive}" -C "$tmp" || err "extract failed"
[ -f "${tmp}/${BINARY}" ] || err "binary not found in archive"
chmod +x "${tmp}/${BINARY}"

# --- choose install dir ----------------------------------------------------
if [ -z "$INSTALL_DIR" ]; then
  if [ -w "/usr/local/bin" ] 2>/dev/null; then INSTALL_DIR="/usr/local/bin"
  else INSTALL_DIR="${HOME}/.local/bin"; fi
fi
mkdir -p "$INSTALL_DIR"
mv "${tmp}/${BINARY}" "${INSTALL_DIR}/${BINARY}"

printf 'install: %s installed to %s/%s\n' "$BINARY" "$INSTALL_DIR" "$BINARY"
case ":${PATH}:" in
  *":${INSTALL_DIR}:"*) ;;
  *) printf 'install: add %s to your PATH:\n  export PATH="%s:$PATH"\n' "$INSTALL_DIR" "$INSTALL_DIR" ;;
esac
"${INSTALL_DIR}/${BINARY}" --version 2>/dev/null || true
