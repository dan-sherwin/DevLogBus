# Daemon

`devlogbusd` is the local broker. It accepts structured records, keeps a bounded
in-memory replay buffer per source, serves live subscriptions, and hosts the
embedded browser UI.

## Runtime Model

`devlogbusd run` starts three boundaries:

- Broker endpoint for Go SDK and CLI clients.
- Optional additional TCP listener for remote troubleshooting.
- HTTP listener for the browser UI, Browser Tap, HTTP publishing, replay, and
  Server-Sent Events.

Records are not written to disk. Restarting the daemon clears the in-memory
replay buffer.

## Defaults

| Setting | macOS/Linux | Windows |
| --- | --- | --- |
| Broker endpoint | `/tmp/devlogbus/devlogbus.sock` | `127.0.0.1:7422` |
| HTTP listener | `127.0.0.1:7423` | `127.0.0.1:7423` |
| Extra TCP listener | disabled | disabled |
| Replay retention | 5000 records per source | 5000 records per source |
| Echo to stdout | enabled | enabled |

## Run

```bash
devlogbusd run
```

Common options:

```bash
devlogbusd run --endpoint /tmp/devlogbus/devlogbus.sock
devlogbusd run --endpoint 127.0.0.1:7422
devlogbusd run --http 127.0.0.1:7424
devlogbusd run --http ""
devlogbusd run --max-records 1000
devlogbusd run --echo=false
```

Expose the broker to a trusted machine during active troubleshooting:

```bash
devlogbusd run --endpoint 0.0.0.0:7422 --http 0.0.0.0:7423
```

Do not expose DevLogBus listeners to the internet. DevLogBus is a local
developer tool and does not implement authentication.

## Settings

`devlogbusd` uses persisted settings for workstation convenience:

```bash
devlogbusd settings list active
devlogbusd settings set endpoint /tmp/devlogbus/devlogbus.sock
devlogbusd settings set endpoint 127.0.0.1:7422
devlogbusd settings set http_listen_address 127.0.0.1:7423
devlogbusd settings set tcp_listen_address 127.0.0.1:7422
devlogbusd settings set max_records 5000
devlogbusd settings set echo true
```

Command-line flags override persisted settings for that process invocation.

## Health And About

The HTTP listener exposes:

```bash
curl http://127.0.0.1:7423/api/health
curl http://127.0.0.1:7423/api/about
```

`/api/health` returns `{"ok":true}` when the HTTP process is alive.

`/api/about` returns API status, broker endpoint settings, retention settings,
and build information. The browser UI About button reads this endpoint.

## Replay Buffer

The daemon retains up to `max_records` records per source. A noisy source should
not push quiet sources out of replay. New subscribers can request:

- `replay`: number of matching records across the filtered replay set.
- `replayPerSource`: number of records to replay for each source.

The terminal UI defaults to replaying up to 1000 records per source on connect.

## Clear Versus Expunge

Viewer clear operations only remove records from that viewer's current in-memory
screen state.

Expunge deletes records from the daemon replay buffer:

```bash
devlogbus expunge --source demo
devlogbus expunge --all
curl -X DELETE 'http://127.0.0.1:7423/api/records/expunge?source=demo'
```

Expunge does not recall records that have already been delivered to live
subscribers.

## Linux Service

Linux builds provide systemd service commands:

```bash
sudo devlogbusd systemd install
sudo systemctl start devlogbusd.service
sudo systemctl status devlogbusd.service
```

See [Linux](linux.md).

## Troubleshooting

Check the configured endpoint:

```bash
devlogbus endpoint
devlogbusd settings list active
```

Check that the HTTP listener is reachable:

```bash
curl http://127.0.0.1:7423/api/health
```

If CLI clients cannot connect, verify that they are using the same endpoint as
the daemon. On Windows, the default is TCP. On macOS and Linux, the default is a
Unix socket.
