# Examples

These examples assume `devlogbusd` is running with the default HTTP listener:

```bash
devlogbusd run
```

Open the browser UI:

```text
http://127.0.0.1:7423/
```

## Go slog

See `examples/go-slog/main.go`.

Run from the repository:

```bash
go run ./examples/go-slog
```

The example uses `pkg/sloghandler` so normal `log/slog` records publish to
DevLogBus without changing call sites.

## C

See `sdk/c/examples/basic.c`.

Build and run from the repository:

```bash
cmake -S sdk/c -B sdk/c/build
cmake --build sdk/c/build
./sdk/c/build/devlogbus_c_basic
```

The example uses the C SDK source package under `sdk/c`.

## .NET / C#

The SDK source package lives under `sdk/dotnet`.

Use the client directly from a .NET app:

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

Run the SDK test program from the repository when the .NET SDK is installed:

```bash
dotnet run --project sdk/dotnet/DevLogBus.Sdk.Tests
```

## Rust

See `sdk/rust/examples/basic.rs`.

Run from the repository:

```bash
cargo run --manifest-path sdk/rust/Cargo.toml --example basic
```

The example uses the Rust SDK source package under `sdk/rust`.

## Java/Kotlin

See `sdk/jvm/examples/Basic.java`.

Build and run from the repository:

```bash
mkdir -p sdk/jvm/build/classes
javac -d sdk/jvm/build/classes \
  $(find sdk/jvm/src/main/java sdk/jvm/examples -name '*.java')
java -cp sdk/jvm/build/classes Basic
```

The example uses the Java-first JVM SDK source package under `sdk/jvm`. Kotlin
code can call the same public classes directly.

## Node/TypeScript

See `examples/node-typescript/devlogbus.ts`.

Run with a TypeScript runner:

```bash
npx tsx examples/node-typescript/devlogbus.ts
```

The example uses the SDK source package under `sdk/node`.

## Python

See `examples/python/devlogbus_example.py`.

Run:

```bash
python3 examples/python/devlogbus_example.py
```

The example uses the SDK source package under `sdk/python`.

## Browser/React Workflow

See `examples/browser-workflow/index.html`.

Open the file in Chrome or serve it with any local static server. Attach Browser
Tap to the tab, then click the buttons. DevLogBus should show:

- direct browser console records from Browser Tap
- runtime exception records from Browser Tap
- network request/response records from Browser Tap
- one direct HTTP record posted by the page itself

This is the core DevLogBus workflow: application records, browser console
records, runtime failures, and network records visible in one timeline.

## Direct HTTP Publish

Publish one record with curl:

```bash
curl -X POST http://127.0.0.1:7423/api/records \
  -H 'Content-Type: application/json' \
  -d '{"level":"info","source":"curl","message":"hello from curl"}'
```

Publish a Browser Tap-shaped record:

```bash
curl -X POST http://127.0.0.1:7423/api/records \
  -H 'Content-Type: application/json' \
  -d '{
    "level": "warn",
    "source": "chrome:api.localhost:8080",
    "message": "GET /api/orders -> 500",
    "attrs": {
      "sourceGroup": "chrome:localhost:5173",
      "tabTitle": "Local Checkout",
      "status": 500
    }
  }'
```
