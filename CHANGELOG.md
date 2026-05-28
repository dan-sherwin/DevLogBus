# Changelog

## v1.0.0 - 2026-05-28

Version 1 marks DevLogBus as a usable local-first development log bus for
real-time app debugging.

### Highlights

- Local broker daemon with a stable default Unix socket at
  `/tmp/devlogbus/devlogbus.sock`, optional TCP listener, HTTP API, embedded
  React viewer, replay, SSE streaming, and expunge support.
- CLI tools for emitting records, tailing streams, opening the interactive TUI,
  inspecting settings/build info, and bridging Linux systemd journal records
  into a DevLogBus broker.
- Public Go packages for broker clients, protocol types, and a non-blocking
  `slog.Handler` publisher.
- Browser publishing over HTTP, including the Chrome Browser Tap extension for
  console calls, runtime exceptions, browser log entries, and network request,
  response, and failure records.
- Dynamic source grouping via `sourceGroup`, including Chrome tab parent
  sources, child network target sources, page-title labels, merged/by-source
  group controls, and source/group popout windows with reattach.
- Terminal UI support for merged and by-source layouts, source-group drilldown,
  search, level filters, pause/follow-bottom, details, clear, expunge, and
  in-app help with `?` or `h`.
- Browser tap detach lifecycle records publish as `WARN` messages so the log
  stream clearly shows when Chrome-side capture has stopped.

