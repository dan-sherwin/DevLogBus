# Node And TypeScript SDK

The Node SDK lives in:

```text
sdk/node
```

It publishes to the DevLogBus HTTP API and has no runtime dependencies. The
default endpoint is:

```text
http://127.0.0.1:7423
```

Install from npm:

```bash
npm install @dan-sherwin/devlogbus
```

Install from a checkout or release source archive when developing the SDK
itself:

```bash
npm install /path/to/DevLogBus/sdk/node
```

## Client

```ts
import { DevLogBusClient } from "@dan-sherwin/devlogbus";

const devlog = new DevLogBusClient({
  source: "checkout_api",
});

await devlog.publish({
  level: "INFO",
  message: "checkout started",
  attrs: { requestId: "req-1" },
});
```

Pass `endpoint` explicitly for a different local or trusted-network daemon:

```ts
const devlog = new DevLogBusClient({
  endpoint: "http://devbox:7423",
  source: "checkout_api",
});
```

## Logger Helper

```ts
const logger = devlog.logger();

await logger.warn("payment provider slow", {
  provider: "demo",
  elapsedMs: 812,
});
```

## Filters And Redaction

Filters drop records before publishing. Redactors return the record shape that
will be sent to the daemon.

```ts
import { DevLogBusClient, dropSources, redactAttrs } from "@dan-sherwin/devlogbus";

const devlog = new DevLogBusClient({
  source: "checkout_api",
  filter: dropSources(["noisy_worker"]),
  redactor: redactAttrs(["authorization", "token", "request.apiKey"]),
});
```

`redactAttrs` matches either an attribute key or dotted nested path and replaces
matching values with `[REDACTED]`.

## Local Test

From the repository root:

```bash
npm --prefix sdk/node test
```
