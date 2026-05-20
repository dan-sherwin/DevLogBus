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
