# .NET / C# SDK

The .NET SDK lives in:

```text
sdk/dotnet
```

It publishes to the DevLogBus HTTP API using `HttpClient` and
`System.Text.Json`, so .NET app records can appear beside backend, CLI, browser,
journal, HTTP, and other SDK records. The default endpoint is:

```text
http://127.0.0.1:7423
```

## Scope

The .NET SDK includes:

- async HTTP publish
- structured `Attrs` dictionaries
- caller-provided `AttrsJson`
- filter hook
- redactor hook
- simple logger helper
- package-ready `net8.0` project files

It does not include background queues, socket protocol support, hosted service
registration, or a dependency on `Microsoft.Extensions.Logging`.

## Install

```bash
dotnet add package DanSherwin.DevLogBus.Sdk
```

## Build And Test

```bash
dotnet run --project sdk/dotnet/DevLogBus.Sdk.Tests
```

## Client

```csharp
using DanSherwin.DevLogBus;

var devlog = new DevLogBusClient(new DevLogBusClientOptions
{
    Source = "checkout_api",
});

await devlog.PublishMessageAsync(
    "INFO",
    "checkout started",
    new Dictionary<string, object?> { ["requestId"] = "req-1" });
```

Pass `Endpoint` explicitly for a different local or trusted-network daemon:

```csharp
var devlog = new DevLogBusClient(new DevLogBusClientOptions
{
    Endpoint = "http://devbox:7423",
    Source = "checkout_api",
});
```

## Logger Helper

```csharp
var logger = devlog.Logger();

await logger.WarnAsync(
    "payment provider slow",
    new Dictionary<string, object?> { ["elapsedMs"] = 812 });
```

## Filters And Redaction

Filters drop records before publishing. Redactors return the record shape that
will be sent to the daemon.

```csharp
var devlog = new DevLogBusClient(new DevLogBusClientOptions
{
    Source = "checkout_api",
    Filter = DevLogBusClient.DropSources("noisy_worker"),
    Redactor = DevLogBusClient.RedactMessage(),
});
```

Use `Attrs` for normal structured metadata. Use `AttrsJson` only when the
caller already has a JSON object string and does not want the SDK to rebuild it.
