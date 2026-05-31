export const DEFAULT_HTTP_ENDPOINT = "http://127.0.0.1:7423";
export const REDACTED_VALUE = "[REDACTED]";

const levelAliases = new Map([
  ["debug", "DEBUG"],
  ["dbg", "DEBUG"],
  ["info", "INFO"],
  ["", "INFO"],
  ["warn", "WARN"],
  ["warning", "WARN"],
  ["error", "ERROR"],
  ["err", "ERROR"],
]);

export function normalizeLevel(level = "INFO") {
  const value = String(level ?? "").trim();
  const mapped = levelAliases.get(value.toLowerCase());
  return mapped ?? value.toUpperCase();
}

export function createRecord(input = {}, defaultSource = "") {
  const source = String(input.source ?? defaultSource).trim();
  const message = String(input.message ?? "");
  if (source === "") {
    throw new Error("DevLogBus source is required");
  }
  if (message === "") {
    throw new Error("DevLogBus message is required");
  }

  const record = {
    time: formatTime(input.time),
    level: normalizeLevel(input.level),
    source,
    message,
  };
  if (input.attrs && Object.keys(input.attrs).length > 0) {
    record.attrs = input.attrs;
  }
  return record;
}

export class DevLogBusClient {
  constructor(options = {}) {
    this.endpoint = String(options.endpoint ?? DEFAULT_HTTP_ENDPOINT).replace(/\/+$/, "");
    this.source = String(options.source ?? "");
    this.fetchImpl = options.fetch ?? globalThis.fetch;
    this.filter = options.filter;
    this.redactor = options.redactor;
    if (typeof this.fetchImpl !== "function") {
      throw new Error("DevLogBus requires fetch; pass options.fetch or use Node 18+");
    }
  }

  async publish(input) {
    const prepared = this.prepare(input);
    if (!prepared.publish) {
      return { published: 0, filtered: true };
    }

    const response = await this.fetchImpl(`${this.endpoint}/api/records`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(prepared.record),
    });
    if (!response.ok) {
      throw new Error(`DevLogBus publish failed: ${response.status}`);
    }
    return response.json();
  }

  async publishBatch(records) {
    const prepared = records.map((record) => this.prepare(record)).filter((item) => item.publish);
    if (prepared.length === 0) {
      return { published: 0, filtered: true };
    }

    const response = await this.fetchImpl(`${this.endpoint}/api/records`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ records: prepared.map((item) => item.record) }),
    });
    if (!response.ok) {
      throw new Error(`DevLogBus publish failed: ${response.status}`);
    }
    return response.json();
  }

  logger(source = "") {
    const loggerSource = String(source || this.source);
    const publish = (level, message, attrs = {}) => this.publish({ level, source: loggerSource, message, attrs });
    return {
      debug: (message, attrs = {}) => publish("DEBUG", message, attrs),
      info: (message, attrs = {}) => publish("INFO", message, attrs),
      warn: (message, attrs = {}) => publish("WARN", message, attrs),
      error: (message, attrs = {}) => publish("ERROR", message, attrs),
    };
  }

  prepare(input) {
    const record = createRecord(input, this.source);
    if (this.filter && !this.filter(record)) {
      return { publish: false };
    }
    const out = this.redactor ? this.redactor(record) : record;
    return {
      publish: true,
      record: validateRecord(out),
    };
  }
}

export function createLogger(options = {}) {
  return new DevLogBusClient(options).logger(options.source);
}

export function dropSources(sources = []) {
  const blocked = new Set(sources.map((source) => String(source).trim()).filter(Boolean));
  return (record) => !blocked.has(record.source);
}

export function redactAttrs(keys = [], replacement = REDACTED_VALUE) {
  const matchers = new Set(keys.map((key) => String(key).trim().toLowerCase()).filter(Boolean));
  return (record) => {
    if (!record.attrs || matchers.size === 0) {
      return record;
    }
    return { ...record, attrs: redactMap(record.attrs, "", matchers, replacement) };
  };
}

function formatTime(value) {
  if (value instanceof Date) {
    return value.toISOString();
  }
  if (typeof value === "string" && value.trim() !== "") {
    return value;
  }
  return new Date().toISOString();
}

function validateRecord(record) {
  const source = String(record.source ?? "").trim();
  const message = String(record.message ?? "");
  if (source === "") {
    throw new Error("DevLogBus source is required");
  }
  if (message === "") {
    throw new Error("DevLogBus message is required");
  }
  return {
    ...record,
    source,
    message,
    time: formatTime(record.time),
    level: normalizeLevel(record.level),
  };
}

function redactMap(attrs, prefix, matchers, replacement) {
  const out = {};
  for (const [key, value] of Object.entries(attrs)) {
    const path = prefix ? `${prefix}.${key}` : key;
    if (matchesAttrKey(key, path, matchers)) {
      out[key] = replacement;
      continue;
    }
    if (isPlainObject(value)) {
      out[key] = redactMap(value, path, matchers, replacement);
      continue;
    }
    out[key] = value;
  }
  return out;
}

function matchesAttrKey(key, path, matchers) {
  return matchers.has(key.toLowerCase()) || matchers.has(path.toLowerCase());
}

function isPlainObject(value) {
  return value !== null && typeof value === "object" && !Array.isArray(value) && !(value instanceof Date);
}
