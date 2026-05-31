const endpointInput = document.querySelector("#endpoint");
const sourceOverrideInput = document.querySelector("#sourceOverride");
const captureConsoleInput = document.querySelector("#captureConsole");
const captureRuntimeInput = document.querySelector("#captureRuntime");
const captureLogInput = document.querySelector("#captureLog");
const captureNetworkInput = document.querySelector("#captureNetwork");
const allowPatternsInput = document.querySelector("#allowPatterns");
const denyPatternsInput = document.querySelector("#denyPatterns");
const redactAuthTokensInput = document.querySelector("#redactAuthTokens");
const redactSensitiveParamsInput = document.querySelector("#redactSensitiveParams");
const redactCookiesInput = document.querySelector("#redactCookies");
const attachButton = document.querySelector("#attach");
const detachButton = document.querySelector("#detach");
const statePill = document.querySelector("#statePill");
const tabTitle = document.querySelector("#tabTitle");
const tabUrl = document.querySelector("#tabUrl");
const message = document.querySelector("#message");
let currentAttached = false;

document.addEventListener("DOMContentLoaded", () => {
  void refreshStatus();
});

attachButton.addEventListener("click", () => {
  void sendAction("attachActiveTab");
});

detachButton.addEventListener("click", () => {
  void sendAction("detachActiveTab");
});

for (const input of [
  endpointInput,
  sourceOverrideInput,
  captureConsoleInput,
  captureRuntimeInput,
  captureLogInput,
  captureNetworkInput,
  allowPatternsInput,
  denyPatternsInput,
  redactAuthTokensInput,
  redactSensitiveParamsInput,
  redactCookiesInput,
]) {
  input.addEventListener("change", () => {
    void saveOptions();
  });
}

async function refreshStatus() {
  setBusy(true);
  try {
    renderStatus(await sendMessage({ type: "getStatus" }));
    setMessage("");
  } catch (error) {
    setMessage(String(error?.message ?? error), true);
  } finally {
    setBusy(false);
  }
}

async function sendAction(type) {
  setBusy(true);
  try {
    const response = await sendMessage({ type, options: readOptions() });
    renderStatus(response);
    setMessage(type === "attachActiveTab" ? "Attached." : "Detached.");
  } catch (error) {
    setMessage(String(error?.message ?? error), true);
  } finally {
    setBusy(false);
  }
}

async function saveOptions() {
  try {
    renderStatus(await sendMessage({ type: "saveOptions", options: readOptions() }));
    setMessage("Saved.");
  } catch (error) {
    setMessage(String(error?.message ?? error), true);
  }
}

async function sendMessage(payload) {
  const response = await chrome.runtime.sendMessage(payload);
  if (!response?.ok) {
    throw new Error(response?.error ?? "extension request failed");
  }
  return response;
}

function renderStatus(status) {
  const options = status.options ?? {};
  endpointInput.value = options.endpoint ?? "http://127.0.0.1:7423";
  sourceOverrideInput.value = options.sourceOverride ?? "";
  captureConsoleInput.checked = options.captureConsole !== false;
  captureRuntimeInput.checked = options.captureRuntime !== false;
  captureLogInput.checked = options.captureLog !== false;
  captureNetworkInput.checked = options.captureNetwork !== false;
  allowPatternsInput.value = options.allowPatterns ?? "";
  denyPatternsInput.value = options.denyPatterns ?? "";
  redactAuthTokensInput.checked = options.redactAuthTokens !== false;
  redactSensitiveParamsInput.checked = options.redactSensitiveParams !== false;
  redactCookiesInput.checked = options.redactCookies !== false;

  const attached = Boolean(status.attached);
  currentAttached = attached;
  statePill.textContent = attached ? "Attached" : "Detached";
  statePill.classList.toggle("attached", attached);
  setBusy(false);

  tabTitle.textContent = status.tab?.title || "No active tab";
  tabUrl.textContent = status.tab?.url || "";
}

function readOptions() {
  return {
    endpoint: endpointInput.value.trim(),
    sourceOverride: sourceOverrideInput.value.trim(),
    captureConsole: captureConsoleInput.checked,
    captureRuntime: captureRuntimeInput.checked,
    captureLog: captureLogInput.checked,
    captureNetwork: captureNetworkInput.checked,
    allowPatterns: allowPatternsInput.value.trim(),
    denyPatterns: denyPatternsInput.value.trim(),
    redactAuthTokens: redactAuthTokensInput.checked,
    redactSensitiveParams: redactSensitiveParamsInput.checked,
    redactCookies: redactCookiesInput.checked,
  };
}

function setBusy(busy) {
  attachButton.disabled = busy || currentAttached;
  detachButton.disabled = busy || !currentAttached;
}

function setMessage(text, isError = false) {
  message.textContent = text;
  message.classList.toggle("error", isError);
}
