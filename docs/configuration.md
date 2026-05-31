# Configuration Conventions

DevLogBus should be easy to wire into public tools without hidden magic. The
preferred order is explicit configuration first, then persisted local settings,
with environment variables used only as visible convenience wrappers.

## Endpoint Conventions

Public code should accept an explicit endpoint option:

```go
logger := slog.New(sloghandler.New(sloghandler.Options{
    Source:   "checkout_svc",
    Endpoint: "127.0.0.1:7422",
}))
```

Supported endpoint forms:

```text
/tmp/devlogbus/devlogbus.sock
unix:/tmp/devlogbus/devlogbus.sock
127.0.0.1:7422
tcp://127.0.0.1:7422
```

Platform defaults:

- macOS/Linux: `/tmp/devlogbus/devlogbus.sock`
- Windows: `127.0.0.1:7422`

## Application Settings

DevLogBus binaries persist workstation settings:

```bash
devlogbus settings set endpoint /tmp/devlogbus/devlogbus.sock
devlogbusd settings set http_listen_address 127.0.0.1:7423
```

This is useful for a developer's own machine. Libraries should not assume
DevLogBus settings files exist.

## Environment Variables

Environment variables are acceptable as a thin wrapper at an application's
boundary, but should not be the only configuration path.

Good:

```bash
DEVLOGBUS_ENDPOINT=127.0.0.1:7422 ./my-local-service
```

Then the application passes that value into `pkg/runtime`, `pkg/sloghandler`, or
`pkg/client`.

For C, Node/TypeScript, and Python, pass the HTTP endpoint into the SDK client:

```text
http://127.0.0.1:7423
```

Avoid hidden package-level environment reads in reusable libraries. Public users
should be able to see where the endpoint comes from.

## Source Names

Use stable, human-readable source names:

```text
checkout_svc
billing_svc
tenant_ui
chrome:localhost:5173
journald
```

Guidelines:

- Prefer service or app names over machine-specific paths.
- Keep source names stable across restarts.
- Include host or port when it helps separate multiple local apps.
- Avoid secrets, usernames, tokens, and customer identifiers in source names.

## Source Groups

Publishers can set `attrs.sourceGroup` when multiple child sources belong to
one workflow:

```json
{
  "source": "chrome:api.localhost:8080",
  "attrs": {
    "sourceGroup": "chrome:localhost:5173"
  }
}
```

Browser Tap uses this to group all records for one tab while preserving child
network targets.

## Runtime Controls

Applications that need user-facing enable/disable controls should use
`pkg/runtime`:

```go
devlog := runtime.New(runtime.Options{
    Enabled:  false,
    Source:   "checkout_svc",
    Endpoint: "127.0.0.1:7422",
})

devlog.Enable()
_ = devlog.SetEndpoint("127.0.0.1:7422")
devlog.Disable()
```

Store `Enabled` and `Endpoint` however the application normally stores user
preferences. DevLogBus does not require a specific settings package.
