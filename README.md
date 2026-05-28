# DevLogBus

DevLogBus is a local-first structured log bus for development work.

It is not a retention stack, alerting system, metrics backend, or production observability platform. The first job is simple: let multiple local processes publish structured logs to a broker, then let humans attach viewers that make the live stream readable.

## Layout

```text
cmd/
  devlogbusd/        local broker daemon entrypoint
  devlogbus/         CLI client and interactive TUI entrypoint
  devlogbus-journal-bridge/
                     Linux journald-to-DevLogBus bridge

internal/
  devlogbusd/app/    service-template-style bootstrap app for the daemon
  devlogbusd/ui/     React live viewer embedded into devlogbusd
  devlogbus/app/     cli-template-style bootstrap app for the CLI
  journalbridge/app/ journald bridge app
  completions/       shared shell-completion installer helpers
  recordfmt/         shared human log formatting

extensions/
  chrome-devlogbus/  Chrome extension that publishes browser debug events

pkg/
  protocol/          wire messages and filtering helpers
  client/            Go client for the broker
  sloghandler/       non-blocking slog.Handler publisher

```

## Quick Start

Start the broker:

```bash
go run ./cmd/devlogbusd run
```

The broker listens on its configured endpoint for Go/CLI clients and on
`127.0.0.1:7423` for browser clients by default. The default broker endpoint
is the stable local Unix socket `/tmp/devlogbus/devlogbus.sock`, but it can also
be a TCP address like `0.0.0.0:7422`.
Disable the browser endpoint with `--http ""`, or set a different address with
`--http 127.0.0.1:7424`. Open `http://127.0.0.1:7423/` to use the embedded live
viewer.

To expose the broker to another machine during active troubleshooting, run the
primary endpoint on TCP:

```bash
go run ./cmd/devlogbusd run --endpoint 0.0.0.0:7422
go run ./cmd/devlogbus tail --endpoint prod-box:7422 --replay 50
```

On a Linux host, bridge systemd journal records into that remote broker:

```bash
go run ./cmd/devlogbus-journal-bridge run --endpoint tcp://devbox:7422 --since now
go run ./cmd/devlogbus-journal-bridge run --endpoint tcp://devbox:7422 --tail 100 --match _SYSTEMD_UNIT=billing.service
```

In another terminal, tail the stream:

```bash
go run ./cmd/devlogbus tail --replay 50
```

Or open the terminal UI:

```bash
go run ./cmd/devlogbus tui
go run ./cmd/devlogbus tui --endpoint prod-box:7422 --replay-per-source 500
```

Inside the TUI, press `?` for the full control reference, including search,
pane layouts, source-group drilldown, level filters, pause/follow-bottom,
details, clear, and expunge controls.

Emit a test record:

```bash
go run ./cmd/devlogbus emit --source demo --level warn --message "catalog unavailable" --attr service=billing_svc
```

Expunge replay records from the broker:

```bash
go run ./cmd/devlogbus expunge --source demo
go run ./cmd/devlogbus expunge --all
```

Run the live browser viewer directly during UI development:

```bash
VITE_DEVLOGBUS_API_URL=http://127.0.0.1:7423 npm --prefix internal/devlogbusd/ui run dev
```

Publish browser records over HTTP:

```bash
curl -X POST http://127.0.0.1:7423/api/records \
  -H 'Content-Type: application/json' \
  -d '{"level":"info","source":"chrome:demo","message":"button clicked"}'
```

Load the Chrome browser tap from `extensions/chrome-devlogbus` to publish
console, runtime, browser log, and network events from the active tab into the
same DevLogBus stream as backend service records.

## Interactive Viewers

The embedded browser UI and the terminal UI both support the two modes that
matter during active debugging:

- `MERGED` shows every selected record in one chronological timeline.
- `BY SOURCE` splits the same stream into source panes.

Browser publishers can include a `sourceGroup` attribute. DevLogBus uses that
to treat one browser tab as a parent source group while still preserving child
sources for console/runtime records and individual network targets. When the
top-level view is merged, child sources are flattened into the same timeline.
When the top-level view is by source, a multi-child browser group acts like a
mini DevLogBus window with its own merged/by-source mode, layout controls,
level filters, pause, clear, expunge, and details controls.

The browser UI can pop a source group or individual source into its own window
and later reattach it to the main layout. The TUI has the same grouping model:
press `enter` on a grouped pane to drill into child sources, and press `esc` or
`backspace` to return to the parent source list. Press `?` or `h` in the TUI for
the full on-screen help reference.

## Go slog Handler

Services can publish directly with `pkg/sloghandler`:

```go
logger := slog.New(sloghandler.New(sloghandler.Options{
    Source: "event_management_svc",
}))
```

For remote troubleshooting, point the handler at the TCP listener:

```go
logger := slog.New(sloghandler.New(sloghandler.Options{
    Source:   "event_management_svc",
    Endpoint: "devbox:7422",
}))
```

The handler uses a bounded queue and drops records when the broker is
unavailable or the queue is full. It keeps a persistent publisher connection and
reconnects after transport errors. Application logging should never block normal
service work.

## Bootstrap Apps

The two Go binaries follow Dan's standard Go templates:

- `devlogbusd` uses the service-style bootstrap: Kong commands, persistent settings, build info, shell completions, and systemd service commands.
- `devlogbus` uses the CLI-style bootstrap: Kong commands, persistent settings, build info, and shell completions.
- `devlogbus-journal-bridge` uses a small CLI bootstrap and streams Linux journald records to any DevLogBus broker endpoint.

Examples:

```bash
go run ./cmd/devlogbusd settings list active
go run ./cmd/devlogbusd settings set endpoint /tmp/devlogbus/devlogbus.sock
go run ./cmd/devlogbusd settings set endpoint 0.0.0.0:7422
go run ./cmd/devlogbus settings set endpoint /tmp/devlogbus/devlogbus.sock
go run ./cmd/devlogbus settings set endpoint prod-box:7422
go run ./cmd/devlogbus buildinfo
```

`devlogbusd` retains `max_records` per source in memory so a noisy process does
not push quiet sources out of the replay buffer. The terminal UI requests replay
records per source as well; use `--replay-per-source` to tune that startup
window.

The importable Go packages stay in the root module for now:

```text
github.com/dan-sherwin/devlogbus/pkg/client
github.com/dan-sherwin/devlogbus/pkg/protocol
github.com/dan-sherwin/devlogbus/pkg/sloghandler
```

Run the local Go quality gate with:

```bash
./dev/ci-local.sh
```
