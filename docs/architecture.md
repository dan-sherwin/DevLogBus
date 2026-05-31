# Architecture

DevLogBus has three cooperating pieces:

- Producers publish structured records.
- `devlogbusd` accepts records over a configured broker endpoint, can optionally listen on an extra TCP address, and keeps a per-source in-memory replay buffer.
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

The default transport on macOS and Linux is newline-delimited JSON over the
stable Unix domain socket `/tmp/devlogbus/devlogbus.sock`. Windows defaults to
the loopback TCP endpoint `127.0.0.1:7422`. The configured broker endpoint can
also be a TCP address such as `0.0.0.0:7422`, and the same publish, subscribe,
and expunge protocol can run on an additional TCP listener for live
troubleshooting across machines.

The daemon HTTP listener is the browser-facing boundary. It serves the embedded
React viewer, exposes replay and SSE streaming endpoints, and accepts published
records from browser tools at `POST /api/records`. Browser publishers use the
same `protocol.Record` shape as Go clients so the merged stream can show client
and server events in one chronological timeline.

Browser publishers can include `sourceGroup` in record attributes. The UI treats
that as a dynamic parent source: top-level merged view still flattens every
record, while by-source view can show a parent source with child sources for
console, runtime, and network target streams.

Source groups and individual child sources can be popped out into separate
browser windows. The popout URL scopes that window to one group or source, while
shared browser storage marks detached targets so the main layout stops rendering
them until they are reattached. A source-group popout keeps the same merged vs.
by-source controls as the main UI, which makes it useful as a focused debugging
surface rather than a passive viewer clone.

The terminal UI uses the same grouping contract with a terminal-native
interaction model. Top-level by-source mode renders source groups as panes;
pressing enter on a grouped pane drills into its child sources, and escape or
backspace returns to the parent source list. Group panes preserve the normal
levels, pause, bottom, details, clear, and expunge controls by keying state on
`group:<name>` and `source:<name>` scopes instead of raw source names. The TUI
also exposes its command reference in-app with `?` or `h` so the controls stay
discoverable without leaving the debugging session.

Chrome sources keep their stable host-based record key, but the UI can render a
friendlier label from `tabTitle`, for example `chrome:Checkout Admin
(localhost:3010)`.

Browser tap detach is treated as operationally important. When Chrome ends a
debugger session for any reason, including the user dismissing the debugger
banner, the extension publishes a `WARN` lifecycle record so the timeline shows
that browser-side capture stopped.

## Go App Shape

The project is one Go module with multiple binaries. That keeps the public
import paths simple while still letting each binary use a small
application-specific bootstrap layer.

- `cmd/devlogbusd` is intentionally tiny and delegates to `internal/devlogbusd/app`.
- `cmd/devlogbus` is intentionally tiny and delegates to `internal/devlogbus/app`.
- `cmd/devlogbus-journal-bridge` is intentionally tiny and delegates to `internal/journalbridge/app`.
- `internal/devlogbusd/ui` owns the React live viewer and its checked-in production bundle for Go embedding.
- `extensions/chrome-devlogbus` owns the load-unpacked Chrome extension for active browser debugging.
- `pkg/protocol`, `pkg/client`, `pkg/sloghandler`, and `pkg/runtime` remain public packages under the root module.

`pkg/sloghandler` is the fixed-configuration slog path: create it with a
source and endpoint, then let it publish non-blocking records for the life of
the process. `pkg/runtime` is the controlled path for applications that need to
toggle DevLogBus or switch endpoints while running. It deliberately stops at
plain Go state and a `slog.Handler`; persistent settings, command parsers, RPC
hooks, and template-specific wiring belong in the consuming app or in adapter
modules layered above it.

The importable packages are not a nested module yet. Splitting them later should be a deliberate versioning decision, not scaffolding drag.
