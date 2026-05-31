#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
EXT_DIR="$ROOT/extensions/chrome-devlogbus"
OUT_DIR="${1:-$ROOT/dist/browser-tap-store}"
VERSION="${VERSION:-}"
RELEASE_STRICT_VERSION="${RELEASE_STRICT_VERSION:-0}"

command -v node >/dev/null 2>&1 || {
	echo "node is required to read and validate the extension manifest" >&2
	exit 1
}
command -v zip >/dev/null 2>&1 || {
	echo "zip is required to package the Chrome Web Store artifact" >&2
	exit 1
}
command -v unzip >/dev/null 2>&1 || {
	echo "unzip is required to validate the Chrome Web Store artifact" >&2
	exit 1
}

manifest_version="$(node -e "console.log(require(process.argv[1]).version)" "$EXT_DIR/manifest.json")"
if [[ -z "$VERSION" ]]; then
	VERSION="$manifest_version"
fi
release_version="${VERSION#v}"
if [[ "$RELEASE_STRICT_VERSION" == "1" && "$manifest_version" != "$release_version" ]]; then
	echo "browser tap manifest version $manifest_version does not match release version $release_version" >&2
	exit 1
fi

safe_version="$(printf "%s" "$VERSION" | sed "s/[^A-Za-z0-9._-]/-/g")"
artifact_base="devlogbus-browser-tap_${safe_version}"
artifact="$OUT_DIR/$artifact_base.zip"
work_dir="$(mktemp -d)"

cleanup() {
	rm -rf "$work_dir"
}
trap cleanup EXIT

stage="$work_dir/stage"
mkdir -p "$stage/icons" "$OUT_DIR"

cp "$EXT_DIR/manifest.json" "$stage/"
cp "$EXT_DIR/service-worker.js" "$stage/"
cp "$EXT_DIR/popup.html" "$stage/"
cp "$EXT_DIR/popup.css" "$stage/"
cp "$EXT_DIR/popup.js" "$stage/"
cp "$EXT_DIR/icons/"*.png "$stage/icons/"

node - "$stage/manifest.json" <<'NODE'
const fs = require("fs");
const manifestPath = process.argv[2];
const manifest = JSON.parse(fs.readFileSync(manifestPath, "utf8"));
const requiredPermissions = ["activeTab", "debugger", "storage", "tabs"];
const permissions = new Set(manifest.permissions || []);
for (const permission of requiredPermissions) {
  if (!permissions.has(permission)) {
    throw new Error(`missing required permission: ${permission}`);
  }
}
const hosts = new Set(manifest.host_permissions || []);
if (hosts.has("<all_urls>")) {
  throw new Error("Chrome Web Store package must not request <all_urls> host access");
}
for (const host of ["http://127.0.0.1/*", "http://localhost/*"]) {
  if (!hosts.has(host)) {
    throw new Error(`missing daemon host permission: ${host}`);
  }
}
for (const size of ["16", "32", "48", "128"]) {
  if (!manifest.icons?.[size]) {
    throw new Error(`missing icon size ${size}`);
  }
}
NODE

rm -f "$artifact"
export COPYFILE_DISABLE=1
(cd "$stage" && zip -X -qr "$artifact" .)

entries="$(unzip -Z1 "$artifact")"
if ! grep -qx "manifest.json" <<<"$entries"; then
	echo "Chrome Web Store package is invalid: manifest.json is not at the zip root" >&2
	exit 1
fi
if grep -Eq '(^|/)(\.DS_Store|__MACOSX)(/|$)' <<<"$entries"; then
	echo "Chrome Web Store package contains macOS metadata" >&2
	exit 1
fi

printf "Browser Tap Chrome Web Store package written to %s\n" "$artifact"
