# Journal Bridge

`devlogbus-journal-bridge` streams Linux systemd-journald records into any
DevLogBus broker endpoint.

It is Linux-only. On macOS and Windows it returns a clear unsupported-platform
error.

## Basic Usage

Start the daemon:

```bash
devlogbusd run
```

Bridge new journal records:

```bash
devlogbus-journal-bridge run --since now
```

Tail the last 100 journal entries and keep following:

```bash
devlogbus-journal-bridge run --tail 100
```

Publish available entries and exit:

```bash
devlogbus-journal-bridge run --tail 100 --once
```

## Remote Broker

Publish to a broker on another trusted machine:

```bash
devlogbus-journal-bridge run --endpoint tcp://devbox:7422 --since now
```

Endpoint options match the Go client and CLI:

```text
/tmp/devlogbus/devlogbus.sock
unix:/tmp/devlogbus/devlogbus.sock
127.0.0.1:7422
tcp://127.0.0.1:7422
```

## Matches

Filter journal input with repeated `--match FIELD=VALUE` values:

```bash
devlogbus-journal-bridge run \
  --match _SYSTEMD_UNIT=billing.service \
  --match PRIORITY=3 \
  --since 10m
```

## Source Mapping

The bridge chooses source names from journal fields in this order:

```text
_SYSTEMD_UNIT
SYSLOG_IDENTIFIER
_COMM
```

Override or extend that order:

```bash
devlogbus-journal-bridge run \
  --source-field SYSLOG_IDENTIFIER \
  --source-field _SYSTEMD_UNIT \
  --source journald
```

`--source` is the fallback when no source field is available.

## Exclusions

The bridge skips its own service records by default to avoid feedback loops.
Additional units and identifiers can be excluded:

```bash
devlogbus-journal-bridge run \
  --exclude-unit noisy.service \
  --exclude-identifier healthcheck
```

## Attributes

By default, the bridge publishes a selected subset of useful journal fields as
record attributes, including cursor, timestamps, priority, unit, identifier,
PID, executable, command line, transport, and code location fields.

Include all available fields:

```bash
devlogbus-journal-bridge run --all-fields
```

Use `--all-fields` carefully; journal entries can contain noisy or sensitive
fields.

## Echo

Print forwarded records locally while publishing:

```bash
devlogbus-journal-bridge run --echo
```

## Reconnect Behavior

The bridge keeps a persistent publisher connection. If publishing fails, it
closes the publisher and retries on later records. Repeated publish failures are
rate-limited in logs by `--reconnect-log-window`.
