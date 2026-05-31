type DevLogBusRecord = {
  time: string;
  level: "DEBUG" | "INFO" | "WARN" | "ERROR";
  source: string;
  message: string;
  attrs?: Record<string, unknown>;
};

const endpoint = "http://127.0.0.1:7423";

async function publish(record: DevLogBusRecord): Promise<void> {
  const response = await fetch(`${endpoint}/api/records`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(record),
  });
  if (!response.ok) {
    throw new Error(`DevLogBus publish failed: ${response.status}`);
  }
}

await publish({
  time: new Date().toISOString(),
  level: "INFO",
  source: "example_node_typescript",
  message: "order accepted",
  attrs: {
    orderId: "demo-order",
    total: 42.5,
  },
});

await publish({
  time: new Date().toISOString(),
  level: "WARN",
  source: "example_node_typescript",
  message: "inventory check slow",
  attrs: {
    elapsedMs: 812,
  },
});
