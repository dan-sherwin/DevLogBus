# DevLogBus Node/TypeScript SDK

Dependency-free Node SDK for publishing records to the DevLogBus HTTP API.

```bash
npm install @dan-sherwin/devlogbus
```

For SDK development from a checkout, use
`npm install /path/to/DevLogBus/sdk/node`.

```ts
import { DevLogBusClient, redactAttrs } from "@dan-sherwin/devlogbus";

const devlog = new DevLogBusClient({
  source: "checkout_api",
  redactor: redactAttrs(["authorization", "token"]),
});

await devlog.publish({
  level: "INFO",
  message: "checkout started",
  attrs: { requestId: "req-1" },
});
```

The default endpoint is `http://127.0.0.1:7423`. Pass `endpoint` explicitly for
other local or trusted-network daemons.

Filters drop records before publishing:

```ts
import { DevLogBusClient, dropSources } from "@dan-sherwin/devlogbus";

const devlog = new DevLogBusClient({
  source: "worker",
  filter: dropSources(["noisy_worker"]),
});
```

Run tests from the repository root:

```bash
npm --prefix sdk/node test
```
