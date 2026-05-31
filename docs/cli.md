# CLI

`devlogbus` is the command-line client for publishing, tailing, expunging, and
opening the terminal UI.

## Version And Build Info

```bash
devlogbus version
devlogbus buildinfo
devlogbusd version
devlogbusd buildinfo
devlogbus-journal-bridge version
```

Use these commands when reporting issues. Release artifacts stamp version,
commit, build date, Go version, and module path.

## Endpoint

```bash
devlogbus endpoint
devlogbus --endpoint 127.0.0.1:7422 endpoint
```

Endpoint values may be:

- Unix socket path: `/tmp/devlogbus/devlogbus.sock`
- `unix:/tmp/devlogbus/devlogbus.sock`
- TCP address: `127.0.0.1:7422`
- `tcp://127.0.0.1:7422`

## Emit

```bash
devlogbus emit \
  --source demo \
  --level warn \
  --message "catalog unavailable" \
  --attr service=billing \
  --attr request_id=abc123
```

`--attr` accepts repeatable `key=value` pairs.

## Tail

```bash
devlogbus tail --replay 50
devlogbus tail --source demo --level warn --replay 10
devlogbus tail --endpoint prod-box:7422 --source billing_svc
```

`tail` prints formatted records until interrupted.

## TUI

```bash
devlogbus tui
devlogbus tui --replay-per-source 500
devlogbus tui --endpoint prod-box:7422
```

See [TUI](tui.md).

## Expunge

```bash
devlogbus expunge --source demo
devlogbus expunge --all
```

Set exactly one of `--source` or `--all`.

## Settings

The CLI stores workstation defaults through the settings command group:

```bash
devlogbus settings list active
devlogbus settings set endpoint /tmp/devlogbus/devlogbus.sock
devlogbus settings set endpoint 127.0.0.1:7422
```

Use explicit command flags when scripting. Use persisted settings for local
developer convenience.

## Shell Completions

```bash
devlogbus autoCompletions install
devlogbus autoCompletions uninstall
devlogbusd autoCompletions install
```

Completion commands are best installed after binaries are in their final path.
