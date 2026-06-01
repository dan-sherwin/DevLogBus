#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${VERSION:-}"
ARTIFACTS_DIR="${ARTIFACTS_DIR:-$ROOT/dist/release}"
OUT_DIR=""
BASE_URL="${BASE_URL:-https://dan-sherwin.github.io/devlogbus-linux-repo}"
GPG_KEY="${GPG_KEY:-BE890A0D4072D25B97D7F84D2BDF255EF7EE0C9B}"
ALPINE_KEY_NAME="${ALPINE_KEY_NAME:-devlogbus@dan-sherwin}"
ALPINE_PRIVATE_KEY="${ALPINE_PRIVATE_KEY:-}"
ALPINE_KEYCHAIN_SERVICE="${ALPINE_KEYCHAIN_SERVICE:-devlogbus.alpine.signing-key}"
ALPINE_KEYCHAIN_ACCOUNT="${ALPINE_KEYCHAIN_ACCOUNT:-devlogbus}"
CREATEREPO_C="${CREATEREPO_C:-}"

usage() {
	cat <<'EOF'
Usage: dev/linux-package-repo.sh --version v1.3.1 --artifacts dist/release --out /path/to/pages-root

Environment:
  BASE_URL                   Published GitHub Pages root URL.
                             Default: https://dan-sherwin.github.io/devlogbus-linux-repo
  GPG_KEY                    GPG fingerprint used for APT and RPM signing.
  ALPINE_PRIVATE_KEY         Path to Alpine RSA private key.
  ALPINE_KEYCHAIN_SERVICE    macOS Keychain service used when ALPINE_PRIVATE_KEY is not set.
  ALPINE_KEYCHAIN_ACCOUNT    macOS Keychain account used when ALPINE_PRIVATE_KEY is not set.
  ALPINE_KEY_NAME            Alpine public key basename, without .rsa.pub.
  CREATEREPO_C               Path to createrepo_c when it is not on PATH.

The artifacts directory must contain the Linux .deb, .rpm, and .apk files from
dev/release-artifacts.sh for the same version.
EOF
}

while [[ $# -gt 0 ]]; do
	case "$1" in
		--version)
			VERSION="${2:-}"
			shift 2
			;;
		--artifacts)
			ARTIFACTS_DIR="${2:-}"
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

if [[ "$ARTIFACTS_DIR" != /* ]]; then
	ARTIFACTS_DIR="$ROOT/$ARTIFACTS_DIR"
fi
if [[ "$OUT_DIR" != /* ]]; then
	OUT_DIR="$ROOT/$OUT_DIR"
fi
if [[ ! -d "$ARTIFACTS_DIR" ]]; then
	echo "artifacts directory not found: $ARTIFACTS_DIR" >&2
	exit 1
fi
if [[ -z "$OUT_DIR" || "$OUT_DIR" == "/" ]]; then
	echo "refusing unsafe output directory: $OUT_DIR" >&2
	exit 1
fi

require_command() {
	local command_name="$1"
	if ! command -v "$command_name" >/dev/null 2>&1; then
		echo "required command not found: $command_name" >&2
		exit 1
	fi
}

sha256_file() {
	local file="$1"
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$file" | awk '{print $1}'
		return
	fi
	shasum -a 256 "$file" | awk '{print $1}'
}

file_size() {
	local file="$1"
	if stat -f %z "$file" >/dev/null 2>&1; then
		stat -f %z "$file"
		return
	fi
	stat -c %s "$file"
}

file_mtime() {
	local file="$1"
	if stat -f %m "$file" >/dev/null 2>&1; then
		stat -f %m "$file"
		return
	fi
	stat -c %Y "$file"
}

find_createrepo() {
	if [[ -n "$CREATEREPO_C" ]]; then
		printf "%s" "$CREATEREPO_C"
		return
	fi
	if command -v createrepo_c >/dev/null 2>&1; then
		command -v createrepo_c
		return
	fi

	local venv_candidate="/tmp/devlogbus-createrepo-venv/lib/python3.9/site-packages/createrepo_c/data/bin/createrepo_c"
	if [[ -x "$venv_candidate" ]]; then
		printf "%s" "$venv_candidate"
		return
	fi

	echo "createrepo_c not found; set CREATEREPO_C=/path/to/createrepo_c" >&2
	exit 1
}

load_alpine_private_key() {
	local target="$1"
	if [[ -n "$ALPINE_PRIVATE_KEY" ]]; then
		cp "$ALPINE_PRIVATE_KEY" "$target"
		chmod 0600 "$target"
		return
	fi

	if ! command -v security >/dev/null 2>&1; then
		echo "ALPINE_PRIVATE_KEY is required when macOS Keychain is unavailable" >&2
		exit 1
	fi

	if ! security find-generic-password \
		-s "$ALPINE_KEYCHAIN_SERVICE" \
		-a "$ALPINE_KEYCHAIN_ACCOUNT" \
		-w >"$target" 2>/dev/null; then
		echo "Alpine signing key not found in Keychain service $ALPINE_KEYCHAIN_SERVICE account $ALPINE_KEYCHAIN_ACCOUNT" >&2
		exit 1
	fi
	if ! grep -q "BEGIN .*PRIVATE KEY" "$target"; then
		local decoded="$target.decoded"
		xxd -r -p "$target" >"$decoded"
		mv "$decoded" "$target"
	fi
	chmod 0600 "$target"
}

sign_with_gpg() {
	local output="$1"
	shift
	gpg --batch --yes --local-user "$GPG_KEY" "$@" "$output"
}

write_apt_release() {
	local apt_root="$1"
	local dist_dir="$apt_root/dists/stable"
	local release="$dist_dir/Release"
	local date
	date="$(LC_ALL=C TZ=UTC date -u +"%a, %d %b %Y %H:%M:%S +0000")"

	cat >"$release" <<EOF
Origin: DevLogBus
Label: DevLogBus
Suite: stable
Codename: stable
Version: ${VERSION#v}
Date: $date
Architectures: amd64 arm64
Components: main
Description: DevLogBus Linux package repository
EOF

	for algorithm in MD5Sum SHA1 SHA256 SHA512; do
		printf "%s:\n" "$algorithm" >>"$release"
		(
			cd "$dist_dir"
			find main -type f \( -name Packages -o -name Packages.gz \) | sort | while read -r file; do
				case "$algorithm" in
					MD5Sum) digest="$(md5sum "$file" | awk '{print $1}')" ;;
					SHA1) digest="$(shasum -a 1 "$file" | awk '{print $1}')" ;;
					SHA256) digest="$(sha256_file "$file")" ;;
					SHA512) digest="$(shasum -a 512 "$file" | awk '{print $1}')" ;;
				esac
				printf " %s %16s %s\n" "$digest" "$(file_size "$file")" "$file"
			done
		) >>"$release"
	done

	gpg --batch --yes --local-user "$GPG_KEY" --clearsign --digest-algo SHA256 \
		-o "$dist_dir/InRelease" "$release"
	gpg --batch --yes --local-user "$GPG_KEY" --armor --detach-sign \
		-o "$dist_dir/Release.gpg" "$release"
}

build_apt_repo() {
	local out="$1"
	local apt_root="$out/apt"
	local pool="$apt_root/pool/main/d/devlogbus"

	mkdir -p "$pool"
	cp "$ARTIFACTS_DIR"/devlogbus_*_amd64.deb "$pool/"
	cp "$ARTIFACTS_DIR"/devlogbus_*_arm64.deb "$pool/"

	for arch in amd64 arm64; do
		local binary_dir="$apt_root/dists/stable/main/binary-$arch"
		mkdir -p "$binary_dir"
		(
			cd "$apt_root"
			dpkg-scanpackages --arch "$arch" pool /dev/null >"$binary_dir/Packages"
			gzip -9c "$binary_dir/Packages" >"$binary_dir/Packages.gz"
		)
	done

	write_apt_release "$apt_root"
}

build_rpm_repo() {
	local out="$1"
	local createrepo
	createrepo="$(find_createrepo)"

	for arch in x86_64 aarch64; do
		local rpm_dir="$out/rpm/$arch"
		mkdir -p "$rpm_dir"
		cp "$ARTIFACTS_DIR"/devlogbus-*-1."$arch".rpm "$rpm_dir/"

		for rpm_file in "$rpm_dir"/*.rpm; do
			rpmsign --addsign \
				--define "_gpg_name $GPG_KEY" \
				--define "__gpg $(command -v gpg)" \
				"$rpm_file" >/dev/null
		done

		"$createrepo" --quiet "$rpm_dir"
		gpg --batch --yes --local-user "$GPG_KEY" --detach-sign \
			-o "$rpm_dir/repodata/repomd.xml.asc" "$rpm_dir/repodata/repomd.xml"
	done

	cat >"$out/rpm/devlogbus.repo" <<EOF
[devlogbus]
name=DevLogBus
baseurl=$BASE_URL/rpm/\$basearch
enabled=1
gpgcheck=0
repo_gpgcheck=0
gpgkey=$BASE_URL/keys/devlogbus-archive-key.asc
EOF
}

build_apk_repo() {
	local out="$1"
	local work="$2"
	local private_key="$work/alpine-private-key.rsa"
	local public_key="$out/keys/$ALPINE_KEY_NAME.rsa.pub"

	load_alpine_private_key "$private_key"
	openssl rsa -in "$private_key" -pubout -out "$public_key" 2>/dev/null

	for arch in x86_64 aarch64; do
		local repo_dir="$out/alpine/$arch"
		mkdir -p "$repo_dir"
		cp "$ARTIFACTS_DIR"/devlogbus_*_"$arch".apk "$repo_dir/"

		python3 - "$repo_dir" "$private_key" "$ALPINE_KEY_NAME.rsa.pub" "$VERSION" <<'PY'
import base64
import gzip
import hashlib
import io
import os
import subprocess
import sys
import tarfile
import time
import zlib
from pathlib import Path

repo_dir = Path(sys.argv[1])
private_key = Path(sys.argv[2])
public_key_name = sys.argv[3]
version = sys.argv[4]


def package_info(path):
    with tarfile.open(path, "r:gz") as archive:
        info = archive.extractfile(".PKGINFO").read().decode()

    values = {}
    repeated = {}
    for raw_line in info.splitlines():
        if not raw_line or raw_line.startswith("#") or " = " not in raw_line:
            continue
        key, value = raw_line.split(" = ", 1)
        repeated.setdefault(key, []).append(value)
        values[key] = value
    return values, repeated


def control_checksum(path):
    data = path.read_bytes()
    decompressor = zlib.decompressobj(16 + zlib.MAX_WBITS)
    decompressor.decompress(data)
    consumed = len(data) - len(decompressor.unused_data)
    digest = hashlib.sha1(data[:consumed]).digest()
    return "Q1" + base64.b64encode(digest).decode()


def write_record(path):
    values, repeated = package_info(path)
    lines = [
        f"C:{control_checksum(path)}",
        f"P:{values['pkgname']}",
        f"V:{values['pkgver']}",
        f"A:{values.get('arch', '')}",
        f"S:{path.stat().st_size}",
        f"I:{values.get('size', '0')}",
        f"T:{values.get('pkgdesc', '')}",
        f"U:{values.get('url', '')}",
        f"L:{values.get('license', '')}",
        f"o:{values.get('origin', values['pkgname'])}",
    ]
    if values.get("maintainer"):
        lines.append(f"m:{values['maintainer']}")
    build_time = values.get("builddate") or str(int(path.stat().st_mtime))
    lines.append(f"t:{build_time}")
    if values.get("commit"):
        lines.append(f"c:{values['commit']}")
    if repeated.get("depend"):
        lines.append("D:" + " ".join(repeated["depend"]))
    if repeated.get("provides"):
        lines.append("p:" + " ".join(repeated["provides"]))
    if repeated.get("install_if"):
        lines.append("i:" + " ".join(repeated["install_if"]))
    return "\n".join(lines) + "\n\n"


index_text = "".join(write_record(path) for path in sorted(repo_dir.glob("*.apk")))
staging = repo_dir / ".apkindex-staging"
staging.mkdir(exist_ok=True)
(staging / "DESCRIPTION").write_text(f"DevLogBus {version} package repository\n")
(staging / "APKINDEX").write_text(index_text)

unsigned_index = repo_dir / "APKINDEX.unsigned.tar.gz"
with tarfile.open(unsigned_index, "w:gz", compresslevel=9) as archive:
    for name in ("DESCRIPTION", "APKINDEX"):
        item = staging / name
        info = archive.gettarinfo(item, arcname=name)
        info.uid = 0
        info.gid = 0
        info.uname = "root"
        info.gname = "root"
        info.mtime = int(time.time())
        with item.open("rb") as handle:
            archive.addfile(info, handle)

signature = subprocess.check_output(
    [
        "openssl",
        "dgst",
        "-sha1",
        "-sign",
        str(private_key),
        str(unsigned_index),
    ]
)

signature_tar = io.BytesIO()
with tarfile.open(fileobj=signature_tar, mode="w") as archive:
    info = tarfile.TarInfo(f".SIGN.RSA.{public_key_name}")
    info.mode = 0o644
    info.uid = 0
    info.gid = 0
    info.uname = "root"
    info.gname = "root"
    info.mtime = int(time.time())
    info.size = len(signature)
    archive.addfile(info, io.BytesIO(signature))

signature_segment = signature_tar.getvalue()
if signature_segment.endswith(b"\0" * 1024):
    signature_segment = signature_segment[:-1024]

signed_index = repo_dir / "APKINDEX.tar.gz"
with gzip.GzipFile(filename="", mode="wb", fileobj=(repo_dir / "APKINDEX.signature.tar.gz").open("wb"), mtime=0) as gz:
    gz.write(signature_segment)

signed_index.write_bytes((repo_dir / "APKINDEX.signature.tar.gz").read_bytes() + unsigned_index.read_bytes())
subprocess.run(
    [
        "openssl",
        "dgst",
        "-sha1",
        "-verify",
        str(repo_dir.parents[1] / "keys" / public_key_name),
        "-signature",
        str(staging / "signature.bin"),
        str(unsigned_index),
    ],
    check=False,
    stdout=subprocess.DEVNULL,
    stderr=subprocess.DEVNULL,
)
(staging / "signature.bin").write_bytes(signature)
subprocess.run(
    [
        "openssl",
        "dgst",
        "-sha1",
        "-verify",
        str(repo_dir.parents[1] / "keys" / public_key_name),
        "-signature",
        str(staging / "signature.bin"),
        str(unsigned_index),
    ],
    check=True,
)
unsigned_index.unlink()
(repo_dir / "APKINDEX.signature.tar.gz").unlink()
for item in staging.iterdir():
    item.unlink()
staging.rmdir()
PY
	done
}

write_pages_docs() {
	local out="$1"
	local gpg_fingerprint
	gpg_fingerprint="$(gpg --with-colons --fingerprint "$GPG_KEY" | awk -F: '$1 == "fpr" { print $10; exit }')"

	cat >"$out/index.md" <<EOF
# DevLogBus Linux Package Repository

This static package repository is published from GitHub Pages and contains
DevLogBus Linux packages for Debian/Ubuntu, Fedora/RHEL/openSUSE style systems,
and Alpine Linux.

Repository base URL:

\`\`\`text
$BASE_URL
\`\`\`

The default install commands favor fast local developer setup. Signed metadata
and keys are published for users who want the higher-assurance path, but
verification is a choice. If you skip it, you own that tradeoff.

DevLogBus provides the tools you need to maintain your own security, but it does
not force you to use them. Use the checksums, signing keys, and verification
instructions as you see fit, because I am not your mother and it is not my job
to make sure you wear a damn helmet. That choice belongs to you.

In short, piss on the electric fence if you want. Just don't act surprised when
physics files a bug report on your ass.

## Debian / Ubuntu

\`\`\`bash
echo "deb [trusted=yes] $BASE_URL/apt stable main" | sudo tee /etc/apt/sources.list.d/devlogbus.list
sudo apt update
sudo apt install devlogbus
\`\`\`

The APT repository metadata is still signed. Users who want signature checks can
install the key and switch the source to \`signed-by\`:

\`\`\`bash
curl -fsSL $BASE_URL/keys/devlogbus-archive-key.asc | sudo gpg --dearmor -o /usr/share/keyrings/devlogbus-archive-keyring.gpg
echo "deb [signed-by=/usr/share/keyrings/devlogbus-archive-keyring.gpg] $BASE_URL/apt stable main" | sudo tee /etc/apt/sources.list.d/devlogbus.list
\`\`\`

## Fedora / RHEL / openSUSE

\`\`\`bash
sudo curl -fsSL -o /etc/yum.repos.d/devlogbus.repo $BASE_URL/rpm/devlogbus.repo
sudo dnf install devlogbus
\`\`\`

Use the same repository file under \`/etc/zypp/repos.d/devlogbus.repo\` and run
\`sudo zypper install devlogbus\` on openSUSE.

The RPM packages and repository metadata are still signed. Users who want
signature checks can import the key and set \`gpgcheck=1\` and
\`repo_gpgcheck=1\` in the repository file:

\`\`\`bash
sudo rpm --import $BASE_URL/keys/devlogbus-archive-key.asc
\`\`\`

## Alpine Linux

\`\`\`sh
echo "$BASE_URL/alpine/\$(apk --print-arch)" | sudo tee -a /etc/apk/repositories
sudo apk update
sudo apk add --allow-untrusted devlogbus
\`\`\`

The Alpine index is still signed. Users who want signature checks can install
the public key and omit \`--allow-untrusted\`:

\`\`\`sh
sudo wget -O /etc/apk/keys/$ALPINE_KEY_NAME.rsa.pub $BASE_URL/keys/$ALPINE_KEY_NAME.rsa.pub
sudo apk add devlogbus
\`\`\`

## Signing Keys

- APT/RPM GPG fingerprint: \`$gpg_fingerprint\`
- Alpine public key: \`$ALPINE_KEY_NAME.rsa.pub\`

## Current Version

\`devlogbus ${VERSION#v}\`
EOF

	cat >"$out/README.md" <"$out/index.md"
	touch "$out/.nojekyll"
	cat >"$out/index.html" <<EOF
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>DevLogBus Linux Package Repository</title>
  <style>
    body { font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; line-height: 1.5; margin: 2rem auto; max-width: 860px; padding: 0 1rem; color: #202124; }
    pre { background: #f6f8fa; border: 1px solid #d0d7de; overflow: auto; padding: 1rem; }
    code { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; }
    a { color: #0969da; }
  </style>
</head>
<body>
  <h1>DevLogBus Linux Package Repository</h1>
  <p>This GitHub Pages repository serves signed DevLogBus packages for APT, RPM, and Alpine APK package managers.</p>
  <p>The default install commands favor fast local developer setup. Signed metadata and keys are published for users who want the higher-assurance path, but verification is a choice. If you skip it, you own that tradeoff.</p>
  <p>DevLogBus provides the tools you need to maintain your own security, but it does not force you to use them. Use the checksums, signing keys, and verification instructions as you see fit, because I am not your mother and it is not my job to make sure you wear a damn helmet. That choice belongs to you.</p>
  <p>In short, piss on the electric fence if you want. Just don't act surprised when physics files a bug report on your ass.</p>

  <h2>Debian / Ubuntu</h2>
  <pre><code>echo "deb [trusted=yes] $BASE_URL/apt stable main" | sudo tee /etc/apt/sources.list.d/devlogbus.list
sudo apt update
sudo apt install devlogbus</code></pre>
  <p>The APT repository metadata is still signed. Users who want signature checks can install the key and switch the source to <code>signed-by</code>.</p>
  <pre><code>curl -fsSL $BASE_URL/keys/devlogbus-archive-key.asc | sudo gpg --dearmor -o /usr/share/keyrings/devlogbus-archive-keyring.gpg
echo "deb [signed-by=/usr/share/keyrings/devlogbus-archive-keyring.gpg] $BASE_URL/apt stable main" | sudo tee /etc/apt/sources.list.d/devlogbus.list</code></pre>

  <h2>Fedora / RHEL / openSUSE</h2>
  <pre><code>sudo curl -fsSL -o /etc/yum.repos.d/devlogbus.repo $BASE_URL/rpm/devlogbus.repo
sudo dnf install devlogbus</code></pre>
  <p>Use the same repository file under <code>/etc/zypp/repos.d/devlogbus.repo</code> and run <code>sudo zypper install devlogbus</code> on openSUSE.</p>
  <p>The RPM packages and repository metadata are still signed. Users who want signature checks can import the key and set <code>gpgcheck=1</code> and <code>repo_gpgcheck=1</code> in the repository file.</p>
  <pre><code>sudo rpm --import $BASE_URL/keys/devlogbus-archive-key.asc</code></pre>

  <h2>Alpine Linux</h2>
  <pre><code>echo "$BASE_URL/alpine/\$(apk --print-arch)" | sudo tee -a /etc/apk/repositories
sudo apk update
sudo apk add --allow-untrusted devlogbus</code></pre>
  <p>The Alpine index is still signed. Users who want signature checks can install the public key and omit <code>--allow-untrusted</code>.</p>
  <pre><code>sudo wget -O /etc/apk/keys/$ALPINE_KEY_NAME.rsa.pub $BASE_URL/keys/$ALPINE_KEY_NAME.rsa.pub
sudo apk add devlogbus</code></pre>

  <h2>Keys</h2>
  <ul>
    <li>APT/RPM GPG fingerprint: <code>$gpg_fingerprint</code></li>
    <li>Alpine public key: <code>$ALPINE_KEY_NAME.rsa.pub</code></li>
  </ul>

  <p>Current version: <code>devlogbus ${VERSION#v}</code></p>
</body>
</html>
EOF
}

write_checksums() {
	local out="$1"
	(
		cd "$out"
		find apt rpm alpine keys -type f | sort | while read -r file; do
			printf "%s  %s\n" "$(sha256_file "$file")" "$file"
		done
	) >"$out/checksums.txt"
}

require_command dpkg-scanpackages
require_command gpg
require_command gzip
require_command md5sum
require_command openssl
require_command rpm
require_command rpmsign
require_command shasum
require_command tar
require_command xxd

work_dir="$(mktemp -d)"
cleanup() {
	rm -rf "$work_dir"
}
trap cleanup EXIT

rm -rf "$OUT_DIR"
mkdir -p "$OUT_DIR/keys"

gpg --batch --yes --armor --export "$GPG_KEY" >"$OUT_DIR/keys/devlogbus-archive-key.asc"

build_apt_repo "$OUT_DIR"
build_rpm_repo "$OUT_DIR"
build_apk_repo "$OUT_DIR" "$work_dir"
write_pages_docs "$OUT_DIR"
write_checksums "$OUT_DIR"

printf "Linux package repository written to %s\n" "$OUT_DIR"
