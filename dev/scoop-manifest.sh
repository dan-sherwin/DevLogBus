#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OWNER="${OWNER:-dan-sherwin}"
REPO="${REPO:-DevLogBus}"
VERSION="${VERSION:-}"
CHECKSUMS="${CHECKSUMS:-$ROOT/dist/release/checksums.txt}"
OUT_FILE=""

usage() {
	cat <<'EOF'
Usage: dev/scoop-manifest.sh --version v1.3.1 --checksums dist/release/checksums.txt [--out bucket/devlogbus.json]

Environment:
  OWNER  GitHub owner or org. Default: dan-sherwin
  REPO   GitHub repository. Default: DevLogBus

The checksum file must come from dev/release-artifacts.sh for the same version.
EOF
}

while [[ $# -gt 0 ]]; do
	case "$1" in
		--version)
			VERSION="${2:-}"
			shift 2
			;;
		--checksums)
			CHECKSUMS="${2:-}"
			shift 2
			;;
		--out)
			OUT_FILE="${2:-}"
			shift 2
			;;
		-h | --help)
			usage
			exit 0
			;;
		*)
			echo "unknown argument: $1" >&2
			usage >&2
			exit 2
			;;
	esac
done

if [[ -z "$VERSION" ]]; then
	echo "--version is required" >&2
	usage >&2
	exit 2
fi
if [[ ! -f "$CHECKSUMS" ]]; then
	echo "checksums file not found: $CHECKSUMS" >&2
	exit 1
fi

safe_version="$(printf "%s" "$VERSION" | sed "s/[^A-Za-z0-9._-]/-/g")"
manifest_version="${VERSION#v}"
release_base="https://github.com/$OWNER/$REPO/releases/download/$VERSION"

checksum_for() {
	local artifact="$1"
	awk -v artifact="$artifact" '$2 == artifact { print $1; found = 1 } END { exit found ? 0 : 1 }' "$CHECKSUMS"
}

windows_amd64_artifact="devlogbus_${safe_version}_windows_amd64.zip"
windows_arm64_artifact="devlogbus_${safe_version}_windows_arm64.zip"
windows_amd64_sha="$(checksum_for "$windows_amd64_artifact")"
windows_arm64_sha="$(checksum_for "$windows_arm64_artifact")"

emit_manifest() {
	cat <<EOF
{
  "version": "$manifest_version",
  "description": "Local-first structured log bus for development work.",
  "homepage": "https://github.com/$OWNER/$REPO",
  "license": "MIT",
  "architecture": {
    "64bit": {
      "url": "$release_base/$windows_amd64_artifact",
      "hash": "$windows_amd64_sha",
      "extract_dir": "devlogbus_${safe_version}_windows_amd64"
    },
    "arm64": {
      "url": "$release_base/$windows_arm64_artifact",
      "hash": "$windows_arm64_sha",
      "extract_dir": "devlogbus_${safe_version}_windows_arm64"
    }
  },
  "bin": [
    "devlogbus.exe",
    "devlogbusd.exe",
    "devlogbus-journal-bridge.exe"
  ],
  "checkver": "github",
  "autoupdate": {
    "architecture": {
      "64bit": {
        "url": "https://github.com/$OWNER/$REPO/releases/download/v\$version/devlogbus_v\$version_windows_amd64.zip",
        "extract_dir": "devlogbus_v\$version_windows_amd64"
      },
      "arm64": {
        "url": "https://github.com/$OWNER/$REPO/releases/download/v\$version/devlogbus_v\$version_windows_arm64.zip",
        "extract_dir": "devlogbus_v\$version_windows_arm64"
      }
    },
    "hash": {
      "url": "https://github.com/$OWNER/$REPO/releases/download/v\$version/checksums.txt"
    }
  }
}
EOF
}

if [[ -n "$OUT_FILE" ]]; then
	mkdir -p "$(dirname "$OUT_FILE")"
	emit_manifest >"$OUT_FILE"
	printf "Scoop manifest written to %s\n" "$OUT_FILE" >&2
else
	emit_manifest
fi
