# Architecture

DevLogBus has three cooperating pieces:

- Producers publish structured records.
- `devlogbusd` accepts records over a configured broker endpoint, can optionally listen on an extra TCP address, and keeps an in-memory ring buffer.
- `devlogbus-journal-bridge` can run on a Linux host and forward systemd journal records into a broker for active troubleshooting.
- Clients subscribe to replay and live records with lightweight filters.
- The browser UI is built from `internal/devlogbusd/ui`, embedded into `devlogbusd`, and served by the daemon's HTTP listener.
- The Chrome browser tap in `extensions/chrome-devlogbus` attaches to a tab and publishes browser debug events to the daemon HTTP listener.

## Non-Goals

- Long-term retention
- Trend analysis
- Alerting
- Cloud ingest
- Production security boundaries

This is a local developer workflow tool. Keep the broker boring and reliable, and let clients decide how fancy the presentation should be.

## Transports

The default transport is newline-delimited JSON over the stable Unix domain
socket `/tmp/devlogbus/devlogbus.sock`. The configured broker endpoint can also
be a TCP address such as `0.0.0.0:7422`, and the same publish, subscribe, and
expunge protocol can run on an additional TCP listener for live troubleshooting
across machines.

The daemon HTTP listener is the browser-facing boundary. It serves the embedded
React viewer, exposes replay and SSE streaming endpoints, and accepts published
records from browser tools at `POST /api/records`. Browser publishers use the
same `protocol.Record` shape as Go clients so the merged stream can show client
and server events in one chronological timeline.

Browser publishers can include `sourceGroup` in record attributes. The UI treats
that as a dynamic parent source: top-level merged view still flattens every
record, while by-source view can show a parent source with child sources for
console, runtime, and network target streams.

Chrome sources keep their stable host-based record key, but the UI can render a
friendlier label from `tabTitle`, for example `chrome:Spacelink Cloud Portal
(localhost:3010)`.

## Go App Shape

The project is one Go module with multiple binaries. That keeps the public import paths simple while still letting each binary use Dan's standard bootstrap-app conventions.

- `cmd/devlogbusd` is intentionally tiny and delegates to `internal/devlogbusd/app`.
- `cmd/devlogbus` is intentionally tiny and delegates to `internal/devlogbus/app`.
- `cmd/devlogbus-journal-bridge` is intentionally tiny and delegates to `internal/journalbridge/app`.
- `internal/devlogbusd/ui` owns the React live viewer and its checked-in production bundle for Go embedding.
- `extensions/chrome-devlogbus` owns the load-unpacked Chrome extension for active browser debugging.
- `pkg/protocol`, `pkg/client`, and `pkg/sloghandler` remain public packages under the root module.

The importable packages are not a nested module yet. Splitting them later should be a deliberate versioning decision, not scaffolding drag.
