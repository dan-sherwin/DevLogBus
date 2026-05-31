# Quickstart

This quickstart uses release binaries. You do not need Go, Node, npm, or a
source checkout.

## Download

Download the matching archive from the GitHub release page:

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

## Install

macOS or Linux:

```bash
mkdir -p ~/devlogbus
tar -xzf devlogbus_<version>_<os>_<arch>.tar.gz -C ~/devlogbus --strip-components 1
export PATH="$HOME/devlogbus:$PATH"
```

Windows PowerShell:

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
