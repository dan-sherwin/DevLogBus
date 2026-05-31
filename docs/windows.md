# Windows

DevLogBus supports Windows as a local development platform for the daemon, CLI,
terminal UI, HTTP API, embedded browser UI, and Browser Tap extension.

## Install From A Release Artifact

Download the matching Windows zip from a GitHub release, then extract it to a
durable directory such as:

```powershell
Expand-Archive .\devlogbus_<version>_windows_arm64.zip C:\DevLogBus
```

Windows ARM systems, including Apple Silicon Parallels VMs, should use the
`windows_arm64` artifact. Intel/AMD Windows systems should use `windows_amd64`.

## Defaults

Windows defaults the Go/CLI broker endpoint to:

```text
127.0.0.1:7422
```

The daemon still serves the browser API and embedded UI at:

```text
127.0.0.1:7423
```

The Unix socket default used on macOS and Linux is intentionally not the Windows
default. Keep Windows on loopback TCP unless you deliberately bind a trusted
network address for troubleshooting.

## Run In The Foreground

```powershell
.\devlogbusd.exe run
.\devlogbus.exe emit --source demo --level info --message "hello from windows"
.\devlogbus.exe tail --source demo --replay 10
```

To expose the broker and browser API during trusted local-network
troubleshooting:

```powershell
.\devlogbusd.exe run --endpoint 0.0.0.0:7422 --http 0.0.0.0:7423
```

DevLogBus is a local-first developer tool, not an internet-facing observability
service. Do not expose those listeners outside trusted development networks.

## Service Management

The `devlogbusd systemd` commands are for Linux service management and are not
supported on Windows. For now, run `devlogbusd.exe run` in a terminal or wire it
into your own Windows service wrapper if you need background startup.

## Journald Bridge

`devlogbus-journal-bridge.exe` is included in the Windows archive for artifact
consistency, but journald capture is Linux-only. Running it on Windows returns a
clear unsupported-platform error.

## Browser Tap

The Browser Tap extension publishes over the daemon HTTP listener. For Chrome
running on the same Windows machine, the default `http://127.0.0.1:7423`
listener is enough.
