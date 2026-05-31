import { DevLogBusClient, redactAttrs } from "../../sdk/node/src/index.js";

const devlog = new DevLogBusClient({
  source: "example_node_typescript",
  redactor: redactAttrs(["authorization", "token"]),
});

await devlog.publish({
  level: "INFO",
  message: "order accepted",
  attrs: {
    orderId: "demo-order",
    total: 42.5,
  },
});

await devlog.publish({
  level: "WARN",
  message: "inventory check slow",
  attrs: {
    elapsedMs: 812,
  },
});
