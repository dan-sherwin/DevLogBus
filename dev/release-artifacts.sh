#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${1:-$ROOT/dist/release}"
VERSION="${VERSION:-$(git -C "$ROOT" describe --tags --always --dirty 2>/dev/null || echo dev)}"
COMMIT="${COMMIT:-$(git -C "$ROOT" rev-parse --short=12 HEAD 2>/dev/null || echo unknown)}"
BUILD_DATE="${BUILD_DATE:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}"
RELEASE_STRICT_VERSION="${RELEASE_STRICT_VERSION:-0}"
GOTOOLCHAIN="${GOTOOLCHAIN:-go1.26.3}"

if [[ "$OUT_DIR" != /* ]]; then
	OUT_DIR="$ROOT/$OUT_DIR"
fi

safe_version="$(printf "%s" "$VERSION" | sed "s/[^A-Za-z0-9._-]/-/g")"
work_dir="$(mktemp -d)"

cleanup() {
	rm -rf "$work_dir"
}
trap cleanup EXIT

if [[ -z "$OUT_DIR" || "$OUT_DIR" == "/" ]]; then
	echo "refusing to use unsafe release output directory: $OUT_DIR" >&2
	exit 1
fi

rm -rf "$OUT_DIR"
mkdir -p "$OUT_DIR"

export COPYFILE_DISABLE=1
export GOTOOLCHAIN
cd "$ROOT"

npm --prefix "$ROOT/internal/devlogbusd/ui" ci
npm --prefix "$ROOT/internal/devlogbusd/ui" run build

ldflags() {
	local package_path="$1"
	printf -- "-s -w -X %s.Version=%s -X %s.Commit=%s -X %s.BuildDate=%s" \
		"$package_path" "$VERSION" \
		"$package_path" "$COMMIT" \
		"$package_path" "$BUILD_DATE"
}

build_binary() {
	local goos="$1"
	local goarch="$2"
	local binary="$3"
	local package="$4"
	local package_path="$5"
	local stage="$6"
	local suffix=""

	if [[ "$goos" == "windows" ]]; then
		suffix=".exe"
	fi

	GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 \
		go build -trimpath -ldflags "$(ldflags "$package_path")" -o "$stage/$binary$suffix" "$package"
}

archive_stage() {
	local artifact_base="$1"
	local goos="$2"

	if [[ "$goos" == "windows" ]]; then
		(cd "$work_dir" && zip -X -qr "$OUT_DIR/$artifact_base.zip" "$artifact_base")
		return
	fi

	(cd "$work_dir" && tar --no-xattrs -czf "$OUT_DIR/$artifact_base.tar.gz" "$artifact_base")
}

build_target() {
	local goos="$1"
	local goarch="$2"
	local artifact_base="devlogbus_${safe_version}_${goos}_${goarch}"
	local stage="$work_dir/$artifact_base"

	mkdir -p "$stage"
	cp "$ROOT/README.md" "$ROOT/CHANGELOG.md" "$ROOT/LICENSE" "$stage/"
	cp -R "$ROOT/docs" "$stage/"

	build_binary "$goos" "$goarch" "devlogbusd" "./cmd/devlogbusd" "github.com/dan-sherwin/devlogbus/internal/devlogbusd/app/consts" "$stage"
	build_binary "$goos" "$goarch" "devlogbus" "./cmd/devlogbus" "github.com/dan-sherwin/devlogbus/internal/devlogbus/app/consts" "$stage"
	build_binary "$goos" "$goarch" "devlogbus-journal-bridge" "./cmd/devlogbus-journal-bridge" "github.com/dan-sherwin/devlogbus/internal/journalbridge/app/consts" "$stage"

	archive_stage "$artifact_base" "$goos"
}

package_browser_tap() {
	VERSION="$VERSION" RELEASE_STRICT_VERSION="$RELEASE_STRICT_VERSION" "$ROOT/dev/browser-tap-store-package.sh" "$OUT_DIR"
}

checksum_file() {
	local file="$1"

	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$file"
		return
	fi
	shasum -a 256 "$file"
}

write_checksums() {
	(
		cd "$OUT_DIR"
		for artifact in *; do
			[[ "$artifact" == "checksums.txt" ]] && continue
			checksum_file "$artifact"
		done | sort -k 2 >checksums.txt
	)
}

build_target darwin amd64
build_target darwin arm64
build_target linux amd64
build_target linux arm64
build_target windows amd64
build_target windows arm64
package_browser_tap
write_checksums

printf "Release artifacts written to %s\n" "$OUT_DIR"
