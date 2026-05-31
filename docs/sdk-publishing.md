# SDK Publishing

DevLogBus ships source packages for several language ecosystems. The package
metadata should stay aligned with the release tag used for publishing.

## npm

Package:

```text
@dan-sherwin/devlogbus
```

Publish from the repository root:

```bash
cd sdk/node
npm publish --access public
```

The package is scoped, so `--access public` is required for first publish.

## PyPI

Package:

```text
devlogbus
```

Build and publish:

```bash
cd sdk/python
python3 -m build
python3 -m twine upload dist/*
```

For GitHub Actions publishing, prefer PyPI Trusted Publishing instead of a
long-lived token.

## crates.io

Package:

```text
devlogbus
```

Dry run first, then publish:

```bash
cargo publish --manifest-path sdk/rust/Cargo.toml --dry-run
cargo publish --manifest-path sdk/rust/Cargo.toml
```

## NuGet

Package:

```text
DanSherwin.DevLogBus.Sdk
```

Build and publish:

```bash
dotnet pack sdk/dotnet/DevLogBus.Sdk/DevLogBus.Sdk.csproj -c Release
dotnet nuget push sdk/dotnet/DevLogBus.Sdk/bin/Release/DanSherwin.DevLogBus.Sdk.1.3.1.nupkg \
  --api-key "$NUGET_API_KEY" \
  --source https://api.nuget.org/v3/index.json
```

## Maven Central

Coordinates:

```text
io.github.dan-sherwin:devlogbus
```

The JVM SDK includes a Maven Central-ready `pom.xml`, but publishing still
requires a verified Central Portal namespace, token credentials, and a GPG key:

```bash
cd sdk/jvm
mvn deploy
```

If using the `io.github.dan-sherwin` namespace, verify that the Central Portal
has granted access for the matching GitHub namespace before publishing.
