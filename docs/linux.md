# Linux

DevLogBus supports Linux as a first-class local development platform. The
release artifacts include:

- `devlogbusd`: local broker daemon, HTTP API, and embedded browser UI.
- `devlogbus`: CLI client and terminal UI.
- `devlogbus-journal-bridge`: journald-to-DevLogBus bridge.

## Install From A Package Repository

APT:

```bash
echo "deb [trusted=yes] https://dan-sherwin.github.io/devlogbus-linux-repo/apt stable main" | sudo tee /etc/apt/sources.list.d/devlogbus.list
sudo apt update
sudo apt install devlogbus
```

DNF/RPM:

```bash
sudo curl -fsSL -o /etc/yum.repos.d/devlogbus.repo https://dan-sherwin.github.io/devlogbus-linux-repo/rpm/devlogbus.repo
sudo dnf install devlogbus
```

Alpine:

```bash
echo "https://dan-sherwin.github.io/devlogbus-linux-repo/alpine/$(apk --print-arch)" | sudo tee -a /etc/apk/repositories
sudo apk update
sudo apk add --allow-untrusted devlogbus
```

See [Package Managers](package-managers.md) for Homebrew on Linux, direct DEB,
RPM, and APK package installs, and optional package signature verification.

## Install From A Release Artifact

Download the matching Linux archive from a GitHub release, then install the
binaries into a durable executable path:

```bash
tar -xzf devlogbus_<version>_linux_amd64.tar.gz
cd devlogbus_<version>_linux_amd64
sudo install -m 0755 devlogbus devlogbusd devlogbus-journal-bridge /usr/local/bin/
```

Do not install the systemd service from a temporary extraction path such as
`/tmp`. Some Linux hosts mount or label temporary paths so systemd cannot execute
services from them. Move the binaries into `/usr/local/bin`, `/opt/devlogbus`,
or another durable executable location first.

## Defaults

The default broker endpoint is:

```text
/tmp/devlogbus/devlogbus.sock
```

`devlogbusd` creates that Unix socket with broad local write permissions so
normal local development tools can publish records without running as root. The
default HTTP listener is:

```text
127.0.0.1:7423
```

Root-run settings are stored below `/var/lib/<app>/settings.db`. User-run
settings use the normal per-user config directory reported by Go's
`os.UserConfigDir`.

## Run In The Foreground

```bash
devlogbusd run
devlogbus emit --source demo --level info --message "hello from linux"
devlogbus tail --source demo --replay 10
```

To expose the broker and browser API during trusted local-network
troubleshooting:

```bash
devlogbusd run --endpoint 0.0.0.0:7422 --http 0.0.0.0:7423
```

Keep those listeners scoped to trusted development networks. DevLogBus is a
local-first developer tool, not an internet-facing observability service.

## Systemd Service

Install and manage the daemon with:

```bash
sudo devlogbusd systemd install
sudo devlogbusd systemd start
sudo devlogbusd systemd status
sudo devlogbusd systemd restart
sudo devlogbusd systemd stop
sudo devlogbusd systemd remove
```

The generated unit runs `devlogbusd run` as a foreground `Type=simple` service
and relies on systemd for lifecycle management.

## Journald Bridge

Forward journald records into a running broker:

```bash
devlogbus-journal-bridge run --since now
devlogbus-journal-bridge run --tail 100 --match _SYSTEMD_UNIT=devlogbusd.service
```

The bridge uses journald fields such as `_SYSTEMD_UNIT`, `SYSLOG_IDENTIFIER`,
and `_COMM` to choose the DevLogBus source. Use `--source-field` to override or
prioritize fields for a specific troubleshooting session.

## Browser Tap

The Browser Tap extension publishes over the daemon HTTP listener. For a browser
running on the same Linux machine, the default `http://127.0.0.1:7423` listener
is enough. For a browser on another workstation, bind `--http` to a reachable
trusted address and point the extension at that URL.
