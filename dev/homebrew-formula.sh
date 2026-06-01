#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OWNER="${OWNER:-dan-sherwin}"
REPO="${REPO:-DevLogBus}"
VERSION="${VERSION:-}"
CHECKSUMS="${CHECKSUMS:-$ROOT/dist/release/checksums.txt}"
FORMULA_LICENSE="${FORMULA_LICENSE:-MIT}"
OUT_FILE=""

usage() {
	cat <<'EOF'
Usage: dev/homebrew-formula.sh --version v1.3.1 --checksums dist/release/checksums.txt [--out Formula/devlogbus.rb]

Environment:
  OWNER             GitHub owner or org. Default: dan-sherwin
  REPO              GitHub repository. Default: DevLogBus
  FORMULA_LICENSE  Ruby formula license value. Default: MIT

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
formula_version="${VERSION#v}"
release_base="https://github.com/$OWNER/$REPO/releases/download/$VERSION"

checksum_for() {
	local artifact="$1"
	awk -v artifact="$artifact" '$2 == artifact { print $1; found = 1 } END { exit found ? 0 : 1 }' "$CHECKSUMS"
}

artifact_name() {
	local goos="$1"
	local goarch="$2"
	printf "devlogbus_%s_%s_%s.tar.gz" "$safe_version" "$goos" "$goarch"
}

darwin_amd64_artifact="$(artifact_name darwin amd64)"
darwin_arm64_artifact="$(artifact_name darwin arm64)"
linux_amd64_artifact="$(artifact_name linux amd64)"
linux_arm64_artifact="$(artifact_name linux arm64)"

darwin_amd64_sha="$(checksum_for "$darwin_amd64_artifact")"
darwin_arm64_sha="$(checksum_for "$darwin_arm64_artifact")"
linux_amd64_sha="$(checksum_for "$linux_amd64_artifact")"
linux_arm64_sha="$(checksum_for "$linux_arm64_artifact")"

formula_license_line() {
	if [[ "$FORMULA_LICENSE" == :* ]]; then
		printf "  license %s\n" "$FORMULA_LICENSE"
		return
	fi
	printf "  license \"%s\"\n" "$FORMULA_LICENSE"
}

emit_formula() {
	cat <<EOF
class Devlogbus < Formula
  desc "Real-time full-stack development log viewer"
  homepage "https://github.com/$OWNER/$REPO"
  version "$formula_version"
$(formula_license_line)

  on_macos do
    if Hardware::CPU.arm?
      url "$release_base/$darwin_arm64_artifact"
      sha256 "$darwin_arm64_sha"
    else
      url "$release_base/$darwin_amd64_artifact"
      sha256 "$darwin_amd64_sha"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "$release_base/$linux_arm64_artifact"
      sha256 "$linux_arm64_sha"
    else
      url "$release_base/$linux_amd64_artifact"
      sha256 "$linux_amd64_sha"
    end
  end

  def install
    bin.install "devlogbus"
    bin.install "devlogbusd"
    bin.install "devlogbus-journal-bridge"
    doc.install "README.md", "CHANGELOG.md", "LICENSE"
    doc.install "docs"
  end

  def caveats
    <<~EOS
      Start the broker:
        devlogbusd run

      Open the embedded browser UI:
        http://127.0.0.1:7423/

      The journald bridge only captures records on Linux.
    EOS
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/devlogbus version")
    assert_match version.to_s, shell_output("#{bin}/devlogbusd version")
    assert_match version.to_s, shell_output("#{bin}/devlogbus-journal-bridge version")
  end
end
EOF
}

if [[ -n "$OUT_FILE" ]]; then
	mkdir -p "$(dirname "$OUT_FILE")"
	emit_formula >"$OUT_FILE"
	printf "Homebrew formula written to %s\n" "$OUT_FILE" >&2
else
	emit_formula
fi
