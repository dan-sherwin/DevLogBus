# DevLogBus

DevLogBus is a local-first structured log bus for development work.

It is not a retention stack, alerting system, metrics backend, or production observability platform. The first job is simple: let multiple local processes publish structured logs to a broker, then let humans attach viewers that make the live stream readable.

## Layout

```text
cmd/
  devlogbusd/        local broker daemon
  devlogbus/         CLI client, future TUI entrypoint

pkg/
  protocol/          wire messages and filtering helpers
  client/            Go client for the broker
  sloghandler/       non-blocking slog.Handler publisher

webapps/
  devlogbus-ui/      future React UI
```

## Quick Start

Start the broker:

```bash
go run ./cmd/devlogbusd
```

In another terminal, tail the stream:

```bash
go run ./cmd/devlogbus tail --replay 50
```

Emit a test record:

```bash
go run ./cmd/devlogbus emit --source demo --level warn --message "catalog unavailable" --attr service=billing_svc
```

## Go slog Handler

Services can publish directly with `pkg/sloghandler`:

```go
logger := slog.New(sloghandler.New(sloghandler.Options{
    Source: "event_management_svc",
}))
```

The handler uses a bounded queue and drops records when the broker is unavailable or the queue is full. Application logging should never block normal service work.
