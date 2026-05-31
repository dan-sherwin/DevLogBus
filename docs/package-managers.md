# Package Managers

DevLogBus release archives are laid out so package-manager recipes can install
the three public binaries directly:

- `devlogbusd`
- `devlogbus`
- `devlogbus-journal-bridge`

Homebrew is the first supported package-manager path. Scoop, winget, apt, rpm,
and apk can be added later if real user demand shows up.

## Homebrew Tap

Create or update a tap repository such as `dan-sherwin/homebrew-devlogbus`, then
generate the formula from release artifacts:

```bash
VERSION=v0.1.0 ./dev/release-artifacts.sh dist/release
dev/homebrew-formula.sh \
  --version v0.1.0 \
  --checksums dist/release/checksums.txt \
  --out /path/to/homebrew-devlogbus/Formula/devlogbus.rb
```

The generated formula selects the right macOS or Linux archive for Intel and
ARM machines, installs the three binaries, and includes a smoke test for
`devlogbus version` and `devlogbusd version`.

After committing the formula to the tap, users can install with:

```bash
brew tap dan-sherwin/devlogbus
brew install devlogbus
devlogbusd run
```

## License Field

`dev/homebrew-formula.sh` defaults to Homebrew's `:cannot_represent` license
marker because this repository does not currently declare a public license. Once
a project license is chosen, pass it explicitly:

```bash
FORMULA_LICENSE=MIT dev/homebrew-formula.sh \
  --version v0.1.0 \
  --checksums dist/release/checksums.txt \
  --out /path/to/homebrew-devlogbus/Formula/devlogbus.rb
```

Do not claim a license in the formula until the repository has the matching
license file.

## Future Windows Packages

Windows users can install from the GitHub release zip for now. Scoop or winget
should wait until the public release flow has stable tags, durable download
URLs, and enough outside users to justify another package surface.

When that happens, reuse the existing Windows archive names:

```text
devlogbus_<version>_windows_amd64.zip
devlogbus_<version>_windows_arm64.zip
```
