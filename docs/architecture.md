# Architecture

DevLogBus has three cooperating pieces:

- Producers publish structured records.
- `devlogbusd` accepts records over a local Unix socket and keeps an in-memory ring buffer.
- Clients subscribe to replay and live records with lightweight filters.

## Non-Goals

- Long-term retention
- Trend analysis
- Alerting
- Cloud ingest
- Production security boundaries

This is a local developer workflow tool. Keep the broker boring and reliable, and let clients decide how fancy the presentation should be.

## Initial Transport

The first transport is newline-delimited JSON over a Unix domain socket. The protocol is intentionally plain enough to inspect with normal tools and stable enough for Go handlers, CLIs, TUIs, and browser bridges to share.

## Go App Shape

The project is one Go module with multiple binaries. That keeps the public import paths simple while still letting each binary use Dan's standard bootstrap-app conventions.

- `cmd/devlogbusd` is intentionally tiny and delegates to `internal/devlogbusd/app`.
- `cmd/devlogbus` is intentionally tiny and delegates to `internal/devlogbus/app`.
- `pkg/protocol`, `pkg/client`, and `pkg/sloghandler` remain public packages under the root module.

The importable packages are not a nested module yet. Splitting them later should be a deliberate versioning decision, not scaffolding drag.
