import assert from "node:assert/strict";
import { createServer } from "node:http";
import { afterEach, beforeEach, test } from "node:test";
import { DevLogBusClient, REDACTED_VALUE, dropSources, normalizeLevel, redactAttrs } from "../src/index.js";

let server;
let endpoint;
let published;

beforeEach(async () => {
  published = [];
  server = createServer(async (req, res) => {
    if (req.method !== "POST" || req.url !== "/api/records") {
      res.writeHead(404);
      res.end();
      return;
    }
    const chunks = [];
    for await (const chunk of req) {
      chunks.push(chunk);
    }
    published.push(JSON.parse(Buffer.concat(chunks).toString("utf8")));
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ published: 1 }));
  });
  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
  endpoint = `http://127.0.0.1:${server.address().port}`;
});

afterEach(async () => {
  await new Promise((resolve) => server.close(resolve));
});

test("normalizes levels", () => {
  assert.equal(normalizeLevel("warning"), "WARN");
  assert.equal(normalizeLevel("dbg"), "DEBUG");
  assert.equal(normalizeLevel("custom"), "CUSTOM");
});

test("publishes records", async () => {
  const client = new DevLogBusClient({ endpoint, source: "node-test" });

  const result = await client.publish({
    level: "warn",
    message: "hello",
    attrs: { requestId: "req-1" },
  });

  assert.equal(result.published, 1);
  assert.equal(published.length, 1);
  assert.equal(published[0].source, "node-test");
  assert.equal(published[0].level, "WARN");
  assert.equal(published[0].attrs.requestId, "req-1");
});

test("filters before publishing", async () => {
  const client = new DevLogBusClient({
    endpoint,
    source: "hidden",
    filter: dropSources(["hidden"]),
  });

  const result = await client.publish({ message: "drop me" });

  assert.deepEqual(result, { published: 0, filtered: true });
  assert.equal(published.length, 0);
});

test("redacts attrs before publishing", async () => {
  const client = new DevLogBusClient({
    endpoint,
    source: "node-test",
    redactor: redactAttrs(["token", "request.authorization"]),
  });

  await client.publish({
    message: "hello",
    attrs: {
      token: "secret",
      request: {
        authorization: "Bearer secret",
        id: "req-1",
      },
    },
  });

  assert.equal(published[0].attrs.token, REDACTED_VALUE);
  assert.equal(published[0].attrs.request.authorization, REDACTED_VALUE);
  assert.equal(published[0].attrs.request.id, "req-1");
});
