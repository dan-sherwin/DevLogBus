#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OWNER="${OWNER:-dan-sherwin}"
REPO="${REPO:-DevLogBus}"
VERSION="${VERSION:-}"
CHECKSUMS="${CHECKSUMS:-$ROOT/dist/release/checksums.txt}"
OUT_DIR=""

usage() {
	cat <<'EOF'
Usage: dev/winget-manifests.sh --version v1.3.1 --checksums dist/release/checksums.txt --out /path/to/winget-pkgs/manifests/d/DanSherwin/DevLogBus/1.3.1

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
			OUT_DIR="${2:-}"
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

if [[ -z "$VERSION" || -z "$OUT_DIR" ]]; then
	echo "--version and --out are required" >&2
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
	awk -v artifact="$artifact" '$2 == artifact { print toupper($1); found = 1 } END { exit found ? 0 : 1 }' "$CHECKSUMS"
}

windows_amd64_artifact="devlogbus_${safe_version}_windows_amd64.zip"
windows_arm64_artifact="devlogbus_${safe_version}_windows_arm64.zip"
windows_amd64_sha="$(checksum_for "$windows_amd64_artifact")"
windows_arm64_sha="$(checksum_for "$windows_arm64_artifact")"

mkdir -p "$OUT_DIR"

cat >"$OUT_DIR/DanSherwin.DevLogBus.yaml" <<EOF
# yaml-language-server: \$schema=https://aka.ms/winget-manifest.version.1.12.0.schema.json
PackageIdentifier: DanSherwin.DevLogBus
PackageVersion: $manifest_version
DefaultLocale: en-US
ManifestType: version
ManifestVersion: 1.12.0
EOF

cat >"$OUT_DIR/DanSherwin.DevLogBus.locale.en-US.yaml" <<EOF
# yaml-language-server: \$schema=https://aka.ms/winget-manifest.defaultLocale.1.12.0.schema.json
PackageIdentifier: DanSherwin.DevLogBus
PackageVersion: $manifest_version
PackageLocale: en-US
Publisher: Dan Sherwin
PublisherUrl: https://github.com/$OWNER
PublisherSupportUrl: https://github.com/$OWNER/$REPO/issues
PackageName: DevLogBus
PackageUrl: https://github.com/$OWNER/$REPO
License: MIT
LicenseUrl: https://github.com/$OWNER/$REPO/blob/main/LICENSE
ShortDescription: Real-time log viewer for full-stack development.
Description: DevLogBus coalesces backend service logs, CLI/TUI records, Linux journald, browser events, direct HTTP records, and SDK-published records into one live local development stream.
Moniker: devlogbus
Tags:
- logging
- development
- cli
- browser
ManifestType: defaultLocale
ManifestVersion: 1.12.0
EOF

cat >"$OUT_DIR/DanSherwin.DevLogBus.installer.yaml" <<EOF
# yaml-language-server: \$schema=https://aka.ms/winget-manifest.installer.1.12.0.schema.json
PackageIdentifier: DanSherwin.DevLogBus
PackageVersion: $manifest_version
InstallerType: zip
ReleaseDate: 2026-05-31
Installers:
- Architecture: x64
  NestedInstallerType: portable
  NestedInstallerFiles:
  - RelativeFilePath: devlogbus_${safe_version}_windows_amd64\\devlogbus.exe
    PortableCommandAlias: devlogbus
  - RelativeFilePath: devlogbus_${safe_version}_windows_amd64\\devlogbusd.exe
    PortableCommandAlias: devlogbusd
  - RelativeFilePath: devlogbus_${safe_version}_windows_amd64\\devlogbus-journal-bridge.exe
    PortableCommandAlias: devlogbus-journal-bridge
  InstallerUrl: $release_base/$windows_amd64_artifact
  InstallerSha256: $windows_amd64_sha
- Architecture: arm64
  NestedInstallerType: portable
  NestedInstallerFiles:
  - RelativeFilePath: devlogbus_${safe_version}_windows_arm64\\devlogbus.exe
    PortableCommandAlias: devlogbus
  - RelativeFilePath: devlogbus_${safe_version}_windows_arm64\\devlogbusd.exe
    PortableCommandAlias: devlogbusd
  - RelativeFilePath: devlogbus_${safe_version}_windows_arm64\\devlogbus-journal-bridge.exe
    PortableCommandAlias: devlogbus-journal-bridge
  InstallerUrl: $release_base/$windows_arm64_artifact
  InstallerSha256: $windows_arm64_sha
ManifestType: installer
ManifestVersion: 1.12.0
EOF

printf "WinGet manifests written to %s\n" "$OUT_DIR" >&2
