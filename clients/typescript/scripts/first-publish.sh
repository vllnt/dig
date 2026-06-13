#!/usr/bin/env bash
#
# First manual npm publish of @vllnt/dig.
#
# npm Trusted Publishing (the OIDC flow canary.yml uses) can only be configured
# on a package that ALREADY EXISTS. So the very first publish is done by hand,
# with your npm login, to claim the name and create the package — after which
# you configure the Trusted Publisher and CI takes over.
#
# Per the release plan, this first publish goes under the `canary` dist-tag, so
# `latest` stays unset until you cut the first stable vX.Y.Z tag (i.e.
# `npm i @vllnt/dig` stays unresolved; consumers use `@vllnt/dig@canary`).
#
# Prereqs:
#   - `npm login`  (member of the @vllnt org with publish rights)
#   - Go toolchain + pnpm  (the SDK tests run against a real `dig serve`)
#
# Usage:
#   clients/typescript/scripts/first-publish.sh [--dry-run]
#
set -euo pipefail

cd "$(dirname "$0")/.."
PKG_DIR="$(pwd)"
REPO_ROOT="$(git -C "$PKG_DIR" rev-parse --show-toplevel)"
TAG="canary"

DRY_RUN=()
if [ "${1:-}" = "--dry-run" ]; then
	DRY_RUN=(--dry-run)
	echo "DRY RUN — nothing will be published."
fi

step() { printf '\n\033[1m==> %s\033[0m\n' "$1"; }

# ---- 1. Preconditions -------------------------------------------------------
step "Checking preconditions"
for bin in npm pnpm git; do
	command -v "$bin" >/dev/null || { echo "error: '$bin' not found on PATH"; exit 1; }
done
npm whoami >/dev/null 2>&1 || { echo "error: not logged in — run 'npm login' first"; exit 1; }
echo "npm user: $(npm whoami)"

NAME="$(node -p "require('$PKG_DIR/package.json').name")"
if [ "$NAME" != "@vllnt/dig" ]; then
	echo "error: package name is '$NAME', expected '@vllnt/dig'"; exit 1
fi

# This script is for the FIRST publish only; refuse if the package exists.
if npm view "$NAME" version >/dev/null 2>&1; then
	echo "error: $NAME is already published. Use the canary CI (canary.yml) for"
	echo "       subsequent publishes, or cut a vX.Y.Z tag for a stable release."
	exit 1
fi

# ---- 2. Resolve the dig binary the SDK tests drive --------------------------
# Order: an explicit $DIG_BIN, else build from source with Go (discovered even
# if not on PATH). We never grab `dig` off PATH — that name collides with the
# BIND DNS lookup tool.
step "Resolving the dig binary (for the real-daemon tests)"
BUILT_DIR=""
if [ -n "${DIG_BIN:-}" ]; then
	[ -x "$DIG_BIN" ] || { echo "error: DIG_BIN='$DIG_BIN' is not an executable"; exit 1; }
	echo "using DIG_BIN=$DIG_BIN"
else
	GO="$(command -v go || true)"
	for cand in /usr/local/go/bin/go /usr/lib/go/bin/go /opt/go/bin/go "$HOME/go/bin/go"; do
		[ -n "$GO" ] && break
		[ -x "$cand" ] && GO="$cand"
	done
	if [ -z "$GO" ]; then
		echo "error: no dig binary available. Either:"
		echo "  - install Go (https://go.dev/dl) so this can build it, or"
		echo "  - point DIG_BIN at a dig binary, e.g. the latest canary:"
		echo "      gh release download canary -R vllnt/dig -p 'dig_*_linux_amd64.tar.gz'"
		echo "      tar -xzf dig_*_linux_amd64.tar.gz && export DIG_BIN=\"\$PWD/dig\""
		exit 1
	fi
	echo "building dig with $GO"
	BUILT_DIR="$(mktemp -d)"
	DIG_BIN="$BUILT_DIR/dig"
	( cd "$REPO_ROOT" && "$GO" build -o "$DIG_BIN" ./cmd/dig )
fi

# ---- 3. Install, typecheck, build, test (no mocks) --------------------------
step "Installing + typecheck + build"
pnpm install --frozen-lockfile=false
pnpm typecheck
pnpm build

step "Testing against a real dig serve"
DIG_BIN="$DIG_BIN" pnpm test

# ---- 4. Stamp a canary prerelease version -----------------------------------
BASE="$(node -p "require('./package.json').version")"
SHA="$(git -C "$PKG_DIR" rev-parse --short HEAD)"
CANARY="${BASE}-canary.${SHA}"
step "Versioning $NAME@$CANARY (tag: $TAG)"
npm version "$CANARY" --no-git-tag-version --allow-same-version >/dev/null

# Always restore package.json (and clean the temp binary) on exit, even on error.
cleanup() {
	git -C "$PKG_DIR" checkout -- package.json 2>/dev/null || true
	[ -n "$BUILT_DIR" ] && rm -rf "$BUILT_DIR"
}
trap cleanup EXIT

# ---- 5. Publish -------------------------------------------------------------
# Scoped package -> --access public. Provenance is omitted: it requires CI OIDC
# and is added automatically by canary.yml on later publishes.
step "Publishing"
npm publish --tag "$TAG" --access public "${DRY_RUN[@]}"

# ---- 6. Done ----------------------------------------------------------------
step "Done"
cat <<EOF
Published $NAME@$CANARY under the '$TAG' tag.

Consume it:
    npm i $NAME@$TAG

Note: \`npm i $NAME\` (bare) stays unresolved until you publish a stable
vX.Y.Z (the 'latest' tag).

Next, hand future publishes to CI:
  1. npmjs.org -> $NAME -> Settings -> Trusted Publisher:
       GitHub Actions, repo vllnt/dig, workflow canary.yml
  2. gh variable set CANARY_ENABLED --body true
EOF
