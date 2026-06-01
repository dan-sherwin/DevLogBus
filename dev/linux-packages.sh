#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${1:-$ROOT/dist/release}"
VERSION="${VERSION:-$(git -C "$ROOT" describe --tags --always --dirty 2>/dev/null || echo dev)}"
GOTOOLCHAIN="${GOTOOLCHAIN:-go1.26.3}"

if [[ "$OUT_DIR" != /* ]]; then
	OUT_DIR="$ROOT/$OUT_DIR"
fi

safe_version="$(printf "%s" "$VERSION" | sed "s/[^A-Za-z0-9._-]/-/g")"
package_version="${VERSION#v}"
work_dir="$(mktemp -d)"

cleanup() {
	rm -rf "$work_dir"
}
trap cleanup EXIT

export GOTOOLCHAIN

ensure_nfpm() {
	if command -v nfpm >/dev/null 2>&1; then
		return
	fi

	local bin_dir="$work_dir/bin"
	mkdir -p "$bin_dir"
	GOBIN="$bin_dir" go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest
	export PATH="$bin_dir:$PATH"
}

package_arch() {
	local goarch="$1"
	local nfpm_arch="$2"
	local artifact_base="devlogbus_${safe_version}_linux_${goarch}"
	local archive="$OUT_DIR/$artifact_base.tar.gz"
	local extract_dir="$work_dir/extract-$goarch"
	local stage="$extract_dir/$artifact_base"
	local config="$work_dir/nfpm-$goarch.yaml"

	if [[ ! -f "$archive" ]]; then
		echo "linux release archive not found: $archive" >&2
		exit 1
	fi

	mkdir -p "$extract_dir"
	tar -xzf "$archive" -C "$extract_dir"

	cat >"$config" <<EOF
name: devlogbus
arch: $nfpm_arch
platform: linux
version: $package_version
section: utils
priority: optional
maintainer: Dan Sherwin <113303204+dan-sherwin@users.noreply.github.com>
description: |-
  Real-time log viewer for full-stack development. Coalesces backend, CLI/TUI,
  Linux journald, browser, HTTP, and SDK records into one local stream.
vendor: Dan Sherwin
homepage: https://github.com/dan-sherwin/DevLogBus
license: MIT
contents:
  - src: $stage/devlogbus
    dst: /usr/bin/devlogbus
    file_info:
      mode: 0755
  - src: $stage/devlogbusd
    dst: /usr/bin/devlogbusd
    file_info:
      mode: 0755
  - src: $stage/devlogbus-journal-bridge
    dst: /usr/bin/devlogbus-journal-bridge
    file_info:
      mode: 0755
  - src: $stage/README.md
    dst: /usr/share/doc/devlogbus/README.md
  - src: $stage/CHANGELOG.md
    dst: /usr/share/doc/devlogbus/CHANGELOG.md
  - src: $stage/LICENSE
    dst: /usr/share/doc/devlogbus/LICENSE
  - src: $stage/docs
    dst: /usr/share/doc/devlogbus/docs
    type: tree
EOF

	nfpm package --config "$config" --packager deb --target "$OUT_DIR"
	nfpm package --config "$config" --packager rpm --target "$OUT_DIR"
	nfpm package --config "$config" --packager apk --target "$OUT_DIR"
}

ensure_nfpm
package_arch amd64 amd64
package_arch arm64 arm64

printf "Linux native packages written to %s\n" "$OUT_DIR"
