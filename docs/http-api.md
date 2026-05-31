# HTTP API And Wire Protocol

The HTTP API is served by `devlogbusd` on `127.0.0.1:7423` by default. Browser
Tap uses this API, and public tools may use it when an HTTP boundary is simpler
than the Go client socket protocol.

All HTTP JSON responses use `Content-Type: application/json`. The daemon also
sets permissive CORS headers because Browser Tap and local development pages
publish from browser contexts.

## Record Schema

```json
{
  "id": "optional-daemon-assigned-id",
  "time": "2026-05-31T12:00:00Z",
  "level": "INFO",
  "source": "billing_svc",
  "message": "payment method updated",
  "attrs": {
    "request_id": "abc123",
    "tenant": "demo"
  }
}
```

Required fields for publishing:

- `source`
- `message`

Recommended fields:

- `time`
- `level`
- `attrs`

If `time` is omitted through the HTTP API, the zero value is accepted by the
current daemon but publishers should send a real timestamp. Go client publishers
validate that `time` is present.

## Levels

Level names are normalized:

| Input | Stored |
| --- | --- |
| `debug`, `dbg` | `DEBUG` |
| `info`, empty | `INFO` |
| `warn`, `warning` | `WARN` |
| `error`, `err` | `ERROR` |

Unknown levels are uppercased and treated like `INFO` for filtering.

Filtering order:

```text
DEBUG < INFO < WARN < ERROR
```

## Source And Source Groups

`source` is the record owner used for filtering and expunge.

Browser publishers can set `attrs.sourceGroup` to make a parent group while
preserving child sources:

```json
{
  "source": "chrome:api.localhost:8080",
  "message": "GET /api/orders -> 500",
  "attrs": {
    "sourceGroup": "chrome:localhost:5173",
    "tabTitle": "Local Checkout"
  }
}
```

The UI and TUI use `sourceGroup` for grouped panes. The daemon does not give
`sourceGroup` special retention or expunge behavior; expunge still operates on
real sources.

## POST /api/records

Publish one record:

```bash
curl -X POST http://127.0.0.1:7423/api/records \
  -H 'Content-Type: application/json' \
  -d '{"level":"info","source":"demo","message":"hello"}'
```

Publish an array:

```bash
curl -X POST http://127.0.0.1:7423/api/records \
  -H 'Content-Type: application/json' \
  -d '[{"level":"info","source":"demo","message":"one"},{"level":"warn","source":"demo","message":"two"}]'
```

Publish a batch object:

```bash
curl -X POST http://127.0.0.1:7423/api/records \
  -H 'Content-Type: application/json' \
  -d '{"records":[{"level":"info","source":"demo","message":"one"}]}'
```

Response:

```json
{"published":1}
```

Limits:

- Request body limit: 1 MiB.
- Batch limit: 500 records.

## GET /api/records

Replay matching records:

```bash
curl 'http://127.0.0.1:7423/api/records?source=demo&level=warn&replay=20'
```

Query parameters:

| Parameter | Meaning |
| --- | --- |
| `source` | Source to include. Repeatable or comma-separated. |
| `sources` | Alias for multiple sources. Repeatable or comma-separated. |
| `level` | Minimum level. |
| `minLevel` | Alias for `level`. |
| `replay` | Number of matching records to replay. |
| `replayPerSource` | Number of records to replay per source. |
| `replay-per-source` | Alias for `replayPerSource`. |
| `replay_per_source` | Alias for `replayPerSource`. |

Response:

```json
[
  {
    "id": "1",
    "time": "2026-05-31T12:00:00Z",
    "level": "WARN",
    "source": "demo",
    "message": "hello"
  }
]
```

## GET /api/stream

Open a Server-Sent Events stream:

```bash
curl -N 'http://127.0.0.1:7423/api/stream?replay=20'
```

Each event is:

```text
event: record
data: {"id":"1","time":"2026-05-31T12:00:00Z","level":"INFO","source":"demo","message":"hello"}
```

The stream sends replay records first, then live records. A keepalive comment is
sent periodically while the stream is idle:

```text
: keepalive
```

## DELETE /api/records/expunge

Delete replay-buffer records for one source:

```bash
curl -X DELETE 'http://127.0.0.1:7423/api/records/expunge?source=demo'
```

Delete all replay-buffer records by omitting `source`:

```bash
curl -X DELETE http://127.0.0.1:7423/api/records/expunge
```

Response:

```json
{"expunged":12}
```

## GET /api/health

```bash
curl http://127.0.0.1:7423/api/health
```

Response:

```json
{"ok":true}
```

## GET /api/about

```bash
curl http://127.0.0.1:7423/api/about
```

Response includes API status, broker settings, and build metadata:

```json
{
  "api": {"ok": true},
  "broker": {
    "endpoint": "/tmp/devlogbus/devlogbus.sock",
    "httpListenAddress": "127.0.0.1:7423",
    "tcpListenAddress": "",
    "maxRecords": 5000,
    "echo": true
  },
  "build": {
    "app": "devlogbusd",
    "version": "v1.2.0",
    "commit": "abc123",
    "buildDate": "2026-05-31T12:00:00Z"
  }
}
```

## Socket Wire Protocol

The Go client and CLI speak newline-delimited JSON envelopes over Unix sockets
or TCP sockets.

Envelope shape:

```json
{
  "type": "log",
  "record": {
    "time": "2026-05-31T12:00:00Z",
    "level": "INFO",
    "source": "demo",
    "message": "hello"
  }
}
```

Envelope types:

| Type | Direction | Payload |
| --- | --- | --- |
| `log` | client to daemon, daemon to subscribers | `record` |
| `subscribe` | client to daemon | `subscribe` |
| `replay_complete` | daemon to subscriber | none |
| `expunge` | client to daemon | `expunge` |
| `expunge_result` | daemon to client | `expungeResult` |
| `error` | daemon to client | `error` |

Subscribe payload:

```json
{
  "type": "subscribe",
  "subscribe": {
    "sources": ["demo"],
    "minLevel": "WARN",
    "replay": 20,
    "replayPerSource": 100
  }
}
```

Expunge payload:

```json
{
  "type": "expunge",
  "expunge": {"source": "demo"}
}
```

An empty expunge source means all replay records.

## Compatibility Expectations

The v1 API should preserve existing JSON field names and endpoint behavior. New
optional fields may be added. Consumers should ignore unknown fields and avoid
depending on exact object key order.

See [Compatibility](compatibility.md).
