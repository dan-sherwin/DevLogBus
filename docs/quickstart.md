# Quickstart

DevLogBus is made for real-time log viewing during full-stack development. It
coalesces backend/service logs, CLI/TUI records, Chrome browser events, Linux
`journald`, direct HTTP records, and SDK records into one local viewer.

This quickstart uses release binaries or package managers. You do not need Go,
Node, npm, or a source checkout.

## Install

Homebrew on macOS or Linux:

```bash
brew install dan-sherwin/tap/devlogbus
```

Scoop on Windows:

```powershell
scoop bucket add dan-sherwin https://github.com/dan-sherwin/scoop-bucket
scoop install devlogbus
```

Debian or Ubuntu:

```bash
echo "deb [trusted=yes] https://dan-sherwin.github.io/devlogbus-linux-repo/apt stable main" | sudo tee /etc/apt/sources.list.d/devlogbus.list
sudo apt update
sudo apt install devlogbus
```

Fedora, RHEL, Rocky Linux, or compatible systems:

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

When the WinGet manifest has been accepted upstream, Windows users can also
install with:

```powershell
winget install DanSherwin.DevLogBus
```

The package-manager commands install:

```text
devlogbusd
devlogbus
devlogbus-journal-bridge
```

See [Package Managers](package-managers.md) for package repositories, optional
signature verification, and release packaging details.

## Manual Archive Install

You can also download the matching archive from the GitHub release page:

```text
devlogbus_<version>_darwin_arm64.tar.gz
devlogbus_<version>_darwin_amd64.tar.gz
devlogbus_<version>_linux_arm64.tar.gz
devlogbus_<version>_linux_amd64.tar.gz
devlogbus_<version>_windows_arm64.zip
devlogbus_<version>_windows_amd64.zip
```

Use the ARM archive on Apple Silicon Macs and ARM Windows VMs. Use the AMD64
archive on Intel/AMD machines.

macOS or Linux manual install:

```bash
mkdir -p ~/devlogbus
tar -xzf devlogbus_<version>_<os>_<arch>.tar.gz -C ~/devlogbus --strip-components 1
export PATH="$HOME/devlogbus:$PATH"
```

Windows PowerShell manual install:

```powershell
Expand-Archive .\devlogbus_<version>_windows_arm64.zip C:\DevLogBus
$env:PATH = "C:\DevLogBus;$env:PATH"
```

## Start The Daemon

```bash
devlogbusd run
```

The default broker endpoint is:

- macOS/Linux: `/tmp/devlogbus/devlogbus.sock`
- Windows: `127.0.0.1:7422`

The browser UI and HTTP API listen on:

```text
http://127.0.0.1:7423/
```

Open that URL in a browser. The page should show the live DevLogBus viewer.

## Emit A Test Record

In another terminal:

```bash
devlogbus emit --source demo --level warn --message "catalog unavailable" --attr service=billing
```

You should see the record in the browser UI. You can also tail it:

```bash
devlogbus tail --source demo --replay 10
```

## Install Browser Tap Locally

Browser Tap is the Chrome extension that publishes browser console, runtime,
browser log, and network events to `devlogbusd`.

Until the Chrome Web Store listing is live:

1. Download or clone the DevLogBus repository.
2. Open `chrome://extensions`.
3. Enable Developer mode.
4. Click Load unpacked.
5. Select `extensions/chrome-devlogbus`.
6. Open a normal web page.
7. Click the DevLogBus Browser Tap toolbar button.
8. Click Attach.

Trigger a console message or network request in the tab. Browser records should
appear in DevLogBus next to backend and CLI records.

## Stop The Daemon

Foreground runs stop with `Ctrl+C`.

On Linux systems installed as a service:

```bash
sudo systemctl stop devlogbusd.service
```

See [Linux](linux.md) for service install details and [Windows](windows.md) for
Windows platform notes.
