# Package Managers

DevLogBus release archives are laid out so package-manager recipes can install
the three public binaries directly:

- `devlogbusd`
- `devlogbus`
- `devlogbus-journal-bridge`

Homebrew is the first supported package-manager path. Release builds also
produce native Linux packages and helper manifests for Scoop and WinGet.

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

A signed APT or DNF repository can be added later if there is enough demand for
automatic OS-package updates. For now, the GitHub release assets are the package
source of truth.

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
