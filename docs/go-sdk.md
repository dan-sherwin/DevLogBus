# Go SDK

The public Go packages live in the root module:

```text
github.com/dan-sherwin/devlogbus/pkg/protocol
github.com/dan-sherwin/devlogbus/pkg/client
github.com/dan-sherwin/devlogbus/pkg/sloghandler
github.com/dan-sherwin/devlogbus/pkg/runtime
```

They do not require any private app-settings, CLI templates, service templates,
business-specific packages, or organization-private modules.

## slog Handler

Use `pkg/sloghandler` when an application can publish whenever it starts:

```go
package main

import (
    "log/slog"

    "github.com/dan-sherwin/devlogbus/pkg/sloghandler"
)

func main() {
    logger := slog.New(sloghandler.New(sloghandler.Options{
        Source: "checkout_svc",
    }))

    logger.Info("checkout started", slog.String("port", "8080"))
}
```

For remote troubleshooting:

```go
logger := slog.New(sloghandler.New(sloghandler.Options{
    Source:   "checkout_svc",
    Endpoint: "devbox:7422",
}))
```

Behavior:

- The handler is non-blocking.
- Records are queued in a bounded channel.
- Records are dropped if the queue is full.
- Publishing uses a persistent broker connection.
- Connection failures close the current publisher and reconnect on a later
  record.
- Optional filter and redaction hooks run before a queued record leaves the
  process.

## Runtime Controls

Use `pkg/runtime` when an application has UI, CLI, API, or settings controls
that can enable/disable DevLogBus at runtime.

```go
devlog := runtime.New(runtime.Options{
    Enabled:  false,
    Source:   "checkout_svc",
    Endpoint: "127.0.0.1:7422",
})
defer devlog.Close()

logger := slog.New(devlog.Handler())

devlog.Enable()
logger.Warn("payment provider slow", slog.String("provider", "demo"))

_ = devlog.SetEndpoint("devbox:7422")
devlog.Disable()
```

`runtime.Status()` reports enabled state, endpoint, source, generation, and the
last publish error.

Store runtime settings in the host application's normal configuration system.
DevLogBus does not require a specific settings package.

## Client

Use `pkg/client` when building custom publishers, subscribers, or maintenance
tools.

```go
broker := client.New("")
ctx, cancel := context.WithTimeout(context.Background(), time.Second)
defer cancel()

err := broker.Publish(ctx, protocol.Record{
    Time:    time.Now(),
    Level:   "INFO",
    Source:  "custom_tool",
    Message: "published directly",
})
```

Subscribe:

```go
sub, err := broker.Subscribe(ctx, protocol.Subscribe{
    MinLevel:        "WARN",
    ReplayPerSource: 100,
})
if err != nil {
    return err
}
defer sub.Close()

for record := range sub.Records {
    fmt.Println(record.Source, record.Message)
}
```

Expunge:

```go
count, err := broker.Expunge(ctx, "custom_tool")
```

An empty source expunges all daemon replay records.

## Filters And Redaction

Use SDK hooks when an application needs to drop records or redact known
sensitive attributes before records leave the process.

```go
logger := slog.New(sloghandler.New(sloghandler.Options{
    Source:   "checkout_svc",
    Filter:   client.DropSources("noisy_worker"),
    Redactor: client.RedactAttrs("authorization", "token", "request.apiKey"),
}))
```

`client.RedactAttrs` matches either an attribute key or dotted nested path and
replaces matching values with `[REDACTED]`.

The same hooks are available through `pkg/client` and `pkg/runtime` options:

```go
devlog := runtime.New(runtime.Options{
    Enabled:  true,
    Source:   "checkout_svc",
    Redactor: client.RedactAttrs("password"),
})
```

## Protocol

`pkg/protocol` defines records, envelopes, subscription filters, level
normalization, and validation.

Use it when you need stable public structs for HTTP or socket integrations.
