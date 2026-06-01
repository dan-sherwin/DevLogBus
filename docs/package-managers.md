# Package Managers

DevLogBus release archives are laid out so package-manager recipes can install
the three public binaries directly:

- `devlogbusd`
- `devlogbus`
- `devlogbus-journal-bridge`

Supported package-manager paths include Homebrew, native Linux packages and
repositories, Scoop, and WinGet manifests.

## Homebrew Tap

Create or update the tap repository `dan-sherwin/homebrew-tap`, then generate
the formula from release artifacts:

```bash
VERSION=v1.3.1 ./dev/release-artifacts.sh dist/release
dev/homebrew-formula.sh \
  --version v1.3.1 \
  --checksums dist/release/checksums.txt \
  --out /path/to/homebrew-tap/Formula/devlogbus.rb
```

The generated formula selects the right macOS or Linux archive for Intel and
ARM machines, installs the three binaries, and includes a smoke test for
`devlogbus version`, `devlogbusd version`, and
`devlogbus-journal-bridge version`.

After committing the formula to the tap, users can install with:

```bash
brew tap dan-sherwin/tap
brew install devlogbus
devlogbusd run
```

Or in one command:

```bash
brew install dan-sherwin/tap/devlogbus
```

## Native Linux Packages

The release build creates Debian, RPM, and Alpine packages for Linux amd64 and
arm64:

```text
devlogbus_<version>_amd64.deb
devlogbus_<version>_arm64.deb
devlogbus-<version>-1.x86_64.rpm
devlogbus-<version>-1.aarch64.rpm
devlogbus_<version>_x86_64.apk
devlogbus_<version>_aarch64.apk
```

Install directly from a release download:

```bash
sudo apt install ./devlogbus_1.3.1_amd64.deb
sudo dnf install ./devlogbus-1.3.1-1.x86_64.rpm
sudo apk add --allow-untrusted ./devlogbus_1.3.1_x86_64.apk
```

The packages install:

```text
/usr/bin/devlogbus
/usr/bin/devlogbusd
/usr/bin/devlogbus-journal-bridge
```

The Linux package repository generator turns these release assets into signed
APT, RPM, and Alpine repositories that can be hosted from GitHub Pages:

```bash
dev/linux-package-repo.sh \
  --version v1.3.1 \
  --artifacts dist/release \
  --out /path/to/devlogbus-linux-repo
```

The repository uses:

- `keys/devlogbus-archive-key.asc` for APT and RPM metadata/package checks.
- `keys/devlogbus@dan-sherwin.rsa.pub` for Alpine APK index checks.
- `apt/` for Debian and Ubuntu.
- `rpm/` for Fedora, RHEL, and openSUSE-style systems.
- `alpine/` for Alpine Linux.

The published GitHub Pages repository lives at:

```text
https://dan-sherwin.github.io/devlogbus-linux-repo
```

The default install commands favor fast local developer setup. Signed metadata
and keys are published for users who want the higher-assurance path, but
verification is a choice. If you skip it, you own that tradeoff.

DevLogBus provides the tools you need to maintain your own security, but it does
not force you to use them. Use the checksums, signing keys, and verification
instructions as you see fit, because I am not your mother and it is not my job
to make sure you wear a damn helmet. That choice belongs to you.

In short, piss on the electric fence if you want. Just don't act surprised when
physics files a bug report on your ass.

APT:

```bash
echo "deb [trusted=yes] https://dan-sherwin.github.io/devlogbus-linux-repo/apt stable main" | sudo tee /etc/apt/sources.list.d/devlogbus.list
sudo apt update
sudo apt install devlogbus
```

The APT repository metadata is still signed. Users who want signature checks can
install the key and switch the source to `signed-by`:

```bash
curl -fsSL https://dan-sherwin.github.io/devlogbus-linux-repo/keys/devlogbus-archive-key.asc | sudo gpg --dearmor -o /usr/share/keyrings/devlogbus-archive-keyring.gpg
echo "deb [signed-by=/usr/share/keyrings/devlogbus-archive-keyring.gpg] https://dan-sherwin.github.io/devlogbus-linux-repo/apt stable main" | sudo tee /etc/apt/sources.list.d/devlogbus.list
```

DNF/RPM:

```bash
sudo curl -fsSL -o /etc/yum.repos.d/devlogbus.repo https://dan-sherwin.github.io/devlogbus-linux-repo/rpm/devlogbus.repo
sudo dnf install devlogbus
```

For openSUSE, write the same repository file to
`/etc/zypp/repos.d/devlogbus.repo` and run:

```bash
sudo zypper install devlogbus
```

The RPM packages and repository metadata are still signed. Users who want
signature checks can import the key and set `gpgcheck=1` and
`repo_gpgcheck=1` in the repository file:

```bash
sudo rpm --import https://dan-sherwin.github.io/devlogbus-linux-repo/keys/devlogbus-archive-key.asc
```

Alpine:

```sh
echo "https://dan-sherwin.github.io/devlogbus-linux-repo/alpine/$(apk --print-arch)" | sudo tee -a /etc/apk/repositories
sudo apk update
sudo apk add --allow-untrusted devlogbus
```

The Alpine index is still signed. Users who want signature checks can install
the public key and omit `--allow-untrusted`:

```sh
sudo wget -O /etc/apk/keys/devlogbus@dan-sherwin.rsa.pub https://dan-sherwin.github.io/devlogbus-linux-repo/keys/devlogbus@dan-sherwin.rsa.pub
sudo apk add devlogbus
```

## Scoop

Generate the Scoop manifest from release checksums:

```bash
dev/scoop-manifest.sh \
  --version v1.3.1 \
  --checksums dist/release/checksums.txt \
  --out /path/to/scoop-bucket/bucket/devlogbus.json
```

After committing the manifest to a Scoop bucket, users can install with:

```powershell
scoop bucket add dan-sherwin https://github.com/dan-sherwin/scoop-bucket
scoop install devlogbus
```

## WinGet

Generate WinGet manifests from release checksums:

```bash
dev/winget-manifests.sh \
  --version v1.3.1 \
  --checksums dist/release/checksums.txt \
  --out /path/to/winget-pkgs/manifests/d/DanSherwin/DevLogBus/1.3.1
```

The generated manifests target the Windows release zips as portable installers
and expose command aliases for:

```text
devlogbus
devlogbusd
devlogbus-journal-bridge
```

After the manifests are accepted into the Windows Package Manager community
repository, users can install with:

```powershell
winget install DanSherwin.DevLogBus
```

## License Field

`dev/homebrew-formula.sh` defaults the formula license to `MIT`, matching the
repository license file.

Override `FORMULA_LICENSE` only if the project license changes:

```bash
FORMULA_LICENSE=Apache-2.0 dev/homebrew-formula.sh \
  --version v1.3.1 \
  --checksums dist/release/checksums.txt \
  --out /path/to/homebrew-tap/Formula/devlogbus.rb
```

## Windows Release Archive Names

Scoop and WinGet reuse the existing Windows archive names:

```text
devlogbus_<version>_windows_amd64.zip
devlogbus_<version>_windows_arm64.zip
```
