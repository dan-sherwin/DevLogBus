const DEBUGGER_VERSION = "1.3";
const DEFAULT_OPTIONS = {
  endpoint: "http://127.0.0.1:7423",
  sourceOverride: "",
  captureConsole: true,
  captureRuntime: true,
  captureLog: true,
  captureNetwork: true,
};
const MAX_ATTR_TEXT = 2000;
const MAX_MESSAGE_TEXT = 4000;
const FLUSH_INTERVAL_MS = 250;
const FLUSH_BATCH_SIZE = 100;

const attachedTabs = new Map();
const publishQueue = [];
let flushTimer = null;
let cachedOptions = null;

chrome.debugger.onEvent.addListener((source, method, params) => {
  void handleDebuggerEvent(source, method, params ?? {});
});

chrome.debugger.onDetach.addListener((source) => {
  if (source.tabId != null) {
    attachedTabs.delete(source.tabId);
    void setTabBadge(source.tabId, "");
  }
});

chrome.tabs.onRemoved.addListener((tabId) => {
  attachedTabs.delete(tabId);
});

chrome.runtime.onInstalled.addListener(() => {
  void chrome.storage.local.set(DEFAULT_OPTIONS);
  void chrome.action.setBadgeBackgroundColor({ color: "#2563eb" });
});

chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
  void handleRuntimeMessage(message)
    .then((response) => sendResponse({ ok: true, ...response }))
    .catch((error) => sendResponse({ ok: false, error: String(error?.message ?? error) }));
  return true;
});

async function handleRuntimeMessage(message) {
  switch (message?.type) {
    case "getStatus":
      return getPopupStatus();
    case "saveOptions":
      await saveOptions(message.options ?? {});
      return getPopupStatus();
    case "attachActiveTab":
      await saveOptions(message.options ?? {});
      return attachActiveTab();
    case "detachActiveTab":
      return detachActiveTab();
    default:
      throw new Error(`unknown message type: ${message?.type}`);
  }
}

async function getPopupStatus() {
  const options = await loadOptions();
  const tab = await activeTab();
  const attached = tab.id != null ? await isDebuggerAttached(tab.id) : false;
  if (tab.id != null && attached && !attachedTabs.has(tab.id)) {
    attachedTabs.set(tab.id, makeTabState(tab, options));
  }
  return {
    options,
    tab: tabSummary(tab),
    attached,
  };
}

async function attachActiveTab() {
  const options = await loadOptions();
  const tab = await activeTab();
  if (tab.id == null) {
    throw new Error("active tab has no id");
  }
  if (isRestrictedURL(tab.url ?? "")) {
    throw new Error("Chrome does not allow debugger attachment to this page");
  }

  const debuggee = { tabId: tab.id };
  if (!(await isDebuggerAttached(tab.id))) {
    await chrome.debugger.attach(debuggee, DEBUGGER_VERSION);
  }
  attachedTabs.set(tab.id, makeTabState(tab, options));
  await enableDebuggerDomains(debuggee);
  await setTabBadge(tab.id, "ON");
  enqueueRecord(makeLifecycleRecord(tab, options, "attached"));

  return getPopupStatus();
}

async function detachActiveTab() {
  const tab = await activeTab();
  if (tab.id == null) {
    throw new Error("active tab has no id");
  }
  const options = await loadOptions();
  if (await isDebuggerAttached(tab.id)) {
    await chrome.debugger.detach({ tabId: tab.id });
  }
  attachedTabs.delete(tab.id);
  await setTabBadge(tab.id, "");
  enqueueRecord(makeLifecycleRecord(tab, options, "detached"));
  return getPopupStatus();
}

async function activeTab() {
  const tabs = await chrome.tabs.query({ active: true, currentWindow: true });
  if (tabs.length === 0) {
    throw new Error("no active tab");
  }
  return tabs[0];
}

async function isDebuggerAttached(tabId) {
  const targets = await chrome.debugger.getTargets();
  return targets.some((target) => target.tabId === tabId && target.attached);
}

async function enableDebuggerDomains(session) {
  await safeSendCommand(session, "Runtime.enable");
  await safeSendCommand(session, "Log.enable");
  await safeSendCommand(session, "Network.enable");
  await safeSendCommand(session, "Target.setAutoAttach", {
    autoAttach: true,
    waitForDebuggerOnStart: false,
    flatten: true,
  });
}

async function safeSendCommand(session, method, params) {
  try {
    await chrome.debugger.sendCommand(session, method, params);
  } catch (error) {
    console.warn(`DevLogBus debugger command failed: ${method}`, error);
  }
}

async function handleDebuggerEvent(source, method, params) {
  if (source.tabId == null) {
    return;
  }
  const options = await loadOptions();
  const tabState = await stateForTab(source.tabId, options);

  if (method === "Target.attachedToTarget") {
    await enableDebuggerDomains({ ...source, sessionId: params.sessionId });
    return;
  }
  if (method === "Runtime.consoleAPICalled" && options.captureConsole) {
    enqueueRecord(recordFromConsole(tabState, source, params));
    return;
  }
  if (method === "Runtime.exceptionThrown" && options.captureRuntime) {
    enqueueRecord(recordFromException(tabState, source, params));
    return;
  }
  if (method === "Log.entryAdded" && options.captureLog) {
    enqueueRecord(recordFromLogEntry(tabState, source, params.entry ?? {}));
    return;
  }
  if (!options.captureNetwork) {
    return;
  }
  if (method === "Network.requestWillBeSent") {
    handleNetworkRequest(tabState, source, params);
    return;
  }
  if (method === "Network.responseReceived") {
    enqueueRecord(recordFromNetworkResponse(tabState, source, params));
    return;
  }
  if (method === "Network.loadingFailed") {
    enqueueRecord(recordFromNetworkFailure(tabState, source, params));
  }
}

async function stateForTab(tabId, options) {
  const existing = attachedTabs.get(tabId);
  if (existing != null) {
    existing.options = options;
    return existing;
  }
  const tab = await chrome.tabs.get(tabId);
  const next = makeTabState(tab, options);
  attachedTabs.set(tabId, next);
  return next;
}

function makeTabState(tab, options) {
  return {
    tabId: tab.id,
    title: tab.title ?? "",
    url: tab.url ?? "",
    options,
    requests: new Map(),
  };
}

function handleNetworkRequest(tabState, source, params) {
  const request = params.request ?? {};
  if (!params.requestId || shouldSkipURL(request.url, tabState.options.endpoint)) {
    return;
  }
  const entry = {
    method: request.method ?? "GET",
    url: request.url ?? "",
    type: params.type ?? "",
    wallTime: params.wallTime,
    initiator: params.initiator?.type ?? "",
  };
  const target = urlParts(entry.url);
  tabState.requests.set(requestKey(source, params.requestId), entry);
  trimRequestMap(tabState.requests);

  enqueueRecord({
    time: timestampFromWallTime(params.wallTime),
    level: "INFO",
    source: sourceName(tabState, entry.url),
    message: `${entry.method} ${urlForMessage(entry.url)} requested`,
    attrs: baseAttrs(tabState, source, {
      event: "network.request",
      requestId: params.requestId,
      method: entry.method,
      url: entry.url,
      targetHost: target.host,
      targetOrigin: target.origin,
      path: target.path,
      resourceType: entry.type,
      initiator: entry.initiator,
    }),
  });
}

function recordFromNetworkResponse(tabState, source, params) {
  const response = params.response ?? {};
  if (shouldSkipURL(response.url, tabState.options.endpoint)) {
    return null;
  }
  const request = tabState.requests.get(requestKey(source, params.requestId)) ?? {};
  const target = urlParts(response.url);
  const method = request.method ?? "GET";
  const status = Number(response.status ?? 0);
  const level = status >= 500 ? "ERROR" : status >= 400 ? "WARN" : "INFO";
  return {
    level,
    source: sourceName(tabState, response.url),
    message: `${method} ${urlForMessage(response.url)} -> ${status || "response"}`,
    attrs: baseAttrs(tabState, source, {
      event: "network.response",
      requestId: params.requestId,
      method,
      url: response.url,
      targetHost: target.host,
      targetOrigin: target.origin,
      path: target.path,
      status,
      statusText: response.statusText ?? "",
      mimeType: response.mimeType ?? "",
      fromDiskCache: Boolean(response.fromDiskCache),
      resourceType: params.type ?? request.type ?? "",
    }),
  };
}

function recordFromNetworkFailure(tabState, source, params) {
  const request = tabState.requests.get(requestKey(source, params.requestId)) ?? {};
  if (shouldSkipURL(request.url, tabState.options.endpoint)) {
    return null;
  }
  const target = urlParts(request.url);
  return {
    level: params.canceled ? "WARN" : "ERROR",
    source: sourceName(tabState, request.url),
    message: `${request.method ?? "GET"} ${urlForMessage(request.url)} failed: ${params.errorText ?? "network error"}`,
    attrs: baseAttrs(tabState, source, {
      event: "network.failure",
      requestId: params.requestId,
      method: request.method ?? "",
      url: request.url ?? "",
      targetHost: target.host,
      targetOrigin: target.origin,
      path: target.path,
      errorText: params.errorText ?? "",
      canceled: Boolean(params.canceled),
      resourceType: params.type ?? request.type ?? "",
    }),
  };
}

function recordFromConsole(tabState, source, params) {
  const args = Array.isArray(params.args) ? params.args.map(remoteObjectText) : [];
  const topFrame = topCallFrame(params.stackTrace);
  const type = params.type ?? "log";
  return {
    time: timestampFromMilliseconds(params.timestamp),
    level: consoleLevel(type),
    source: sourceName(tabState, topFrame?.url),
    message: truncate(redactText(args.join(" ")), MAX_MESSAGE_TEXT) || `[console.${type}]`,
    attrs: baseAttrs(tabState, source, {
      event: "console",
      consoleType: type,
      args,
      url: topFrame?.url ?? "",
      lineNumber: topFrame?.lineNumber,
      columnNumber: topFrame?.columnNumber,
      executionContextId: params.executionContextId,
    }),
  };
}

function recordFromException(tabState, source, params) {
  const details = params.exceptionDetails ?? {};
  const exception = details.exception ?? {};
  return {
    time: new Date().toISOString(),
    level: "ERROR",
    source: sourceName(tabState, details.url),
    message: truncate(redactText(exception.description ?? details.text ?? "Uncaught exception"), MAX_MESSAGE_TEXT),
    attrs: baseAttrs(tabState, source, {
      event: "runtime.exception",
      text: details.text ?? "",
      exception: remoteObjectText(exception),
      url: details.url ?? "",
      lineNumber: details.lineNumber,
      columnNumber: details.columnNumber,
      stack: stackFrames(details.stackTrace),
    }),
  };
}

function recordFromLogEntry(tabState, source, entry) {
  if (shouldSkipURL(entry.url, tabState.options.endpoint)) {
    return null;
  }
  return {
    time: timestampFromMilliseconds(entry.timestamp),
    level: logEntryLevel(entry.level),
    source: sourceName(tabState, entry.url),
    message: truncate(redactText(entry.text ?? `[${entry.source ?? "browser"}]`), MAX_MESSAGE_TEXT),
    attrs: baseAttrs(tabState, source, {
      event: "browser.log",
      logSource: entry.source ?? "",
      url: entry.url ?? "",
      lineNumber: entry.lineNumber,
      networkRequestId: entry.networkRequestId,
      workerId: entry.workerId,
    }),
  };
}

function makeLifecycleRecord(tab, options, action) {
  const source = options.sourceOverride || defaultSource(tab.url, tab.id);
  return {
    time: new Date().toISOString(),
    level: "INFO",
    source,
    message: `DevLogBus browser tap ${action}`,
    attrs: {
      event: `browser_tap.${action}`,
      sourceGroup: source,
      tabId: tab.id,
      title: tab.title ?? "",
      url: tab.url ?? "",
    },
  };
}

function enqueueRecord(record) {
  if (record == null) {
    return;
  }
  publishQueue.push(record);
  if (publishQueue.length >= FLUSH_BATCH_SIZE) {
    void flushQueue();
    return;
  }
  if (flushTimer == null) {
    flushTimer = setTimeout(() => {
      flushTimer = null;
      void flushQueue();
    }, FLUSH_INTERVAL_MS);
  }
}

async function flushQueue() {
  if (publishQueue.length === 0) {
    return;
  }
  const records = publishQueue.splice(0, FLUSH_BATCH_SIZE);
  const options = await loadOptions();
  const endpoint = normalizeEndpoint(options.endpoint);
  try {
    const response = await fetch(`${endpoint}/api/records`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ records }),
    });
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}`);
    }
  } catch (error) {
    console.warn("DevLogBus publish failed", error);
  }
  if (publishQueue.length > 0) {
    void flushQueue();
  }
}

async function loadOptions() {
  if (cachedOptions != null) {
    return cachedOptions;
  }
  const saved = await chrome.storage.local.get(DEFAULT_OPTIONS);
  cachedOptions = normalizeOptions(saved);
  return cachedOptions;
}

async function saveOptions(options) {
  cachedOptions = normalizeOptions({ ...(await loadOptions()), ...options });
  await chrome.storage.local.set(cachedOptions);
}

function normalizeOptions(options) {
  return {
    endpoint: normalizeEndpoint(options.endpoint || DEFAULT_OPTIONS.endpoint),
    sourceOverride: String(options.sourceOverride ?? "").trim(),
    captureConsole: options.captureConsole !== false,
    captureRuntime: options.captureRuntime !== false,
    captureLog: options.captureLog !== false,
    captureNetwork: options.captureNetwork !== false,
  };
}

function normalizeEndpoint(endpoint) {
  return String(endpoint || DEFAULT_OPTIONS.endpoint).trim().replace(/\/+$/, "");
}

function sourceName(tabState, eventURL) {
  return defaultSource(eventURL || tabState.url, tabState.tabId);
}

function groupName(tabState) {
  return tabState.options.sourceOverride || defaultSource(tabState.url, tabState.tabId);
}

function defaultSource(rawURL, tabId) {
  try {
    const url = new URL(rawURL);
    return `chrome:${url.host}`;
  } catch {
    return `chrome:tab-${tabId ?? "unknown"}`;
  }
}

function baseAttrs(tabState, source, attrs) {
  return sanitizeAttrs({
    ...attrs,
    sourceGroup: groupName(tabState),
    tabId: tabState.tabId,
    tabTitle: tabState.title,
    tabURL: tabState.url,
    debuggerSessionId: source.sessionId ?? "",
    devlogbusPublisher: "chrome-extension",
  });
}

function sanitizeAttrs(attrs) {
  const clean = {};
  for (const [key, value] of Object.entries(attrs)) {
    if (value == null || value === "") {
      continue;
    }
    if (typeof value === "string") {
      clean[key] = truncate(redactText(value), MAX_ATTR_TEXT);
      continue;
    }
    if (Array.isArray(value)) {
      clean[key] = value.map((item) =>
        typeof item === "string" ? truncate(redactText(item), MAX_ATTR_TEXT) : item,
      );
      continue;
    }
    clean[key] = value;
  }
  return clean;
}

function remoteObjectText(value) {
  if (value == null) {
    return "";
  }
  if (value.unserializableValue != null) {
    return String(value.unserializableValue);
  }
  if (Object.prototype.hasOwnProperty.call(value, "value")) {
    if (typeof value.value === "string") {
      return truncate(redactText(value.value), MAX_ATTR_TEXT);
    }
    try {
      return truncate(JSON.stringify(value.value), MAX_ATTR_TEXT);
    } catch {
      return String(value.value);
    }
  }
  if (value.description != null) {
    return truncate(redactText(String(value.description)), MAX_ATTR_TEXT);
  }
  return String(value.type ?? "");
}

function consoleLevel(type) {
  switch (type) {
    case "debug":
      return "DEBUG";
    case "warning":
    case "warn":
      return "WARN";
    case "error":
    case "assert":
      return "ERROR";
    case "info":
    case "log":
    default:
      return "INFO";
  }
}

function logEntryLevel(level) {
  switch (level) {
    case "verbose":
      return "DEBUG";
    case "warning":
      return "WARN";
    case "error":
      return "ERROR";
    case "info":
    default:
      return "INFO";
  }
}

function timestampFromMilliseconds(value) {
  if (typeof value !== "number" || !Number.isFinite(value)) {
    return new Date().toISOString();
  }
  return new Date(value).toISOString();
}

function timestampFromWallTime(value) {
  if (typeof value !== "number" || !Number.isFinite(value)) {
    return new Date().toISOString();
  }
  return new Date(value * 1000).toISOString();
}

function requestKey(source, requestId) {
  return `${source.sessionId ?? "root"}:${requestId}`;
}

function trimRequestMap(requests) {
  while (requests.size > 500) {
    const first = requests.keys().next().value;
    requests.delete(first);
  }
}

function urlForMessage(rawURL) {
  if (!rawURL) {
    return "(unknown)";
  }
  try {
    const url = new URL(rawURL);
    return `${url.pathname || "/"}${url.search}`;
  } catch {
    return rawURL;
  }
}

function urlParts(rawURL) {
  try {
    const url = new URL(rawURL);
    return {
      host: url.host,
      origin: url.origin,
      path: `${url.pathname || "/"}${url.search}`,
    };
  } catch {
    return { host: "", origin: "", path: rawURL ?? "" };
  }
}

function shouldSkipURL(rawURL, endpoint) {
  if (!rawURL) {
    return false;
  }
  return rawURL.startsWith(normalizeEndpoint(endpoint));
}

function topCallFrame(stackTrace) {
  return stackTrace?.callFrames?.[0] ?? null;
}

function stackFrames(stackTrace) {
  return (stackTrace?.callFrames ?? []).slice(0, 8).map((frame) => ({
    functionName: frame.functionName ?? "",
    url: frame.url ?? "",
    lineNumber: frame.lineNumber,
    columnNumber: frame.columnNumber,
  }));
}

function truncate(value, max) {
  const text = String(value ?? "");
  if (text.length <= max) {
    return text;
  }
  return `${text.slice(0, max - 1)}...`;
}

function redactText(value) {
  return String(value)
    .replace(/Bearer\s+[A-Za-z0-9._~+/=-]+/gi, "Bearer [redacted]")
    .replace(/((?:password|passwd|token|secret|api[_-]?key)=)[^&\s]+/gi, "$1[redacted]");
}

function tabSummary(tab) {
  return {
    id: tab.id ?? null,
    title: tab.title ?? "",
    url: tab.url ?? "",
  };
}

function isRestrictedURL(rawURL) {
  return /^(chrome|chrome-extension|edge|about):/i.test(rawURL);
}

async function setTabBadge(tabId, text) {
  try {
    await chrome.action.setBadgeText({ tabId, text });
  } catch {
    // Tab may already be gone.
  }
}
