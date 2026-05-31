# Changelog

## Unreleased

### Highlights

- Added Node/TypeScript, Python, and C SDK source packages for publishing
  records through the DevLogBus HTTP API.
- Added SDK-side record filter and redaction hooks for Go, Node/TypeScript,
  Python, and C publishers.
- Added SDK docs, examples, and local test coverage for the new SDK packages.

## v1.2.0 - 2026-05-31

This release prepares DevLogBus for broader public use.

### Highlights

- Added GitHub Actions release artifacts for macOS, Linux, Windows, checksums,
  and the Browser Tap Chrome Web Store package.
- Verified Linux and Windows release packages with platform smoke coverage.
- Added public package-manager prep, including Homebrew formula generation.
- Fixed release packaging scripts so relative output directories work reliably
  in local runs and GitHub Actions.
- Added Browser Tap controls for local host allow/deny filters, redaction, and
  Chrome Web Store-safe localhost host permissions.
- Added the publish-ready documentation set covering the daemon, browser UI,
  TUI, CLI, HTTP API, wire protocol, Browser Tap, journal bridge, Go SDK,
  security/privacy, configuration, compatibility, examples, and release notes.
- Added public examples for Go `slog`, Node/TypeScript, Python, and browser
  workflows.
- Added GitHub issue templates, contribution notes, a security policy, and
  browser UI screenshot/GIF assets.
- Added the MIT License and release packaging support for carrying the license
  into archives and generated package-manager formulas.

## v1.1.0 - 2026-05-30

### Highlights

- Added public runtime controls for enabling, disabling, and retargeting
  DevLogBus logging from Go applications.
- Kept runtime controls independent from private application settings,
  service-template, CLI-template, RPC, and business-specific packages.
- Added package-level tests for runtime status, endpoint changes, and
  non-blocking handler behavior.

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
