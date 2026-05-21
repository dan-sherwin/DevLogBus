# DevLogBus

DevLogBus is a local-first structured log bus for development work.

It is not a retention stack, alerting system, metrics backend, or production observability platform. The first job is simple: let multiple local processes publish structured logs to a broker, then let humans attach viewers that make the live stream readable.

## Layout

```text
cmd/
  devlogbusd/        local broker daemon entrypoint
  devlogbus/         CLI client, future TUI entrypoint

internal/
  devlogbusd/app/    service-template-style bootstrap app for the daemon
  devlogbusd/ui/     React live viewer embedded into devlogbusd
  devlogbus/app/     cli-template-style bootstrap app for the CLI
  completions/       shared shell-completion installer helpers
  recordfmt/         shared human log formatting

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

The broker listens on the Unix socket for Go/CLI clients and on
`127.0.0.1:7423` for browser clients by default. Disable the browser endpoint
with `--http ""`, or set a different address with `--http 127.0.0.1:7424`.
Open `http://127.0.0.1:7423/` to use the embedded live viewer.

In another terminal, tail the stream:

```bash
go run ./cmd/devlogbus tail --replay 50
```

Emit a test record:

```bash
go run ./cmd/devlogbus emit --source demo --level warn --message "catalog unavailable" --attr service=billing_svc
```

Run the live browser viewer directly during UI development:

```bash
VITE_DEVLOGBUS_API_URL=http://127.0.0.1:7423 npm --prefix internal/devlogbusd/ui run dev
```

## Go slog Handler

Services can publish directly with `pkg/sloghandler`:

```go
logger := slog.New(sloghandler.New(sloghandler.Options{
    Source: "event_management_svc",
}))
```

The handler uses a bounded queue and drops records when the broker is unavailable or the queue is full. Application logging should never block normal service work.

## Bootstrap Apps

The two Go binaries follow Dan's standard Go templates:

- `devlogbusd` uses the service-style bootstrap: Kong commands, persistent settings, build info, shell completions, and systemd service commands.
- `devlogbus` uses the CLI-style bootstrap: Kong commands, persistent settings, build info, and shell completions.

Examples:

```bash
go run ./cmd/devlogbusd settings list active
go run ./cmd/devlogbus settings set socket_path /tmp/devlogbus/devlogbus.sock
go run ./cmd/devlogbus buildinfo
```

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
