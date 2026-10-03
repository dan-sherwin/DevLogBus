import assert from "node:assert/strict";
import { test } from "node:test";
import {
  deleteWorkspace,
  readWorkspaces,
  saveWorkspace,
  workspacesStorageKey,
  type ViewerPreferences,
} from "../src/workspaces.ts";

const preferences: ViewerPreferences = {
  viewMode: "source",
  sourceLayout: "horizontal",
  paneWidth: 560,
  excludedSources: ["group:noise", "source:chrome:analytics"],
  blockedSources: ["chrome:third-party"],
  selectedLevels: ["WARN", "ERROR"],
  perSourceLevels: { "source:portal": [], "group:browser": ["ERROR"] },
  search: "payment",
  mergedAutoScroll: false,
  mergedLineDetails: true,
  autoScrollSources: { "source:portal": false },
  detailSources: { "group:browser": true },
  groupViewModes: { browser: "source" },
  groupLayouts: { browser: "vertical" },
  groupPaneWidths: { browser: 180 },
};

function memoryStorage() {
  const values = new Map<string, string>();
  return {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => { values.set(key, value); },
  };
}

test("all viewer and nested pane preferences survive saving and a fresh read", () => {
  const storage = memoryStorage();
  saveWorkspace("  Payment debugging  ", preferences, false, storage);
  assert.deepEqual(readWorkspaces(storage), [{ name: "Payment debugging", preferences }]);
  assert.equal(JSON.parse(storage.getItem(workspacesStorageKey)!).version, 1);
});

test("malformed or unsupported storage does not crash the viewer", () => {
  const storage = memoryStorage();
  for (const value of ["{", "null", "[]", '{"version":2,"workspaces":[]}', '{"version":1,"workspaces":null}']) {
    storage.setItem(workspacesStorageKey, value);
    assert.deepEqual(readWorkspaces(storage), []);
  }
  assert.deepEqual(readWorkspaces({ ...storage, getItem: () => { throw new Error("Storage disabled"); } }), []);
});

test("invalid entries are skipped while valid workspaces remain loadable", () => {
  const storage = memoryStorage();
  const invalidSettings = [
    { viewMode: "invalid" },
    { sourceLayout: "invalid" },
    { paneWidth: -10 },
    { excludedSources: [null] },
    { blockedSources: "noise" },
    { selectedLevels: ["FATAL"] },
    { perSourceLevels: { portal: "ERROR" } },
    { mergedAutoScroll: null },
    { autoScrollSources: { portal: "false" } },
    { groupViewModes: { browser: "invalid" } },
    { groupLayouts: { browser: null } },
    { groupPaneWidths: { browser: "260" } },
  ];
  storage.setItem(workspacesStorageKey, JSON.stringify({
    version: 1,
    workspaces: [
      null,
      { name: "Missing settings" },
      ...invalidSettings.map((settings, index) => ({ name: `Invalid ${index}`, preferences: { ...preferences, ...settings } })),
      { name: "Payment", preferences },
      { name: " payment ", preferences: { ...preferences, search: "duplicate" } },
    ],
  }));
  assert.deepEqual(readWorkspaces(storage), [{ name: "Payment", preferences }]);
});

test("blank and duplicate names cannot silently replace a saved investigation", () => {
  const storage = memoryStorage();
  saveWorkspace("Payment", preferences, false, storage);
  assert.throws(() => saveWorkspace("  ", preferences, false, storage), /name/);
  assert.throws(() => saveWorkspace(" PAYMENT ", { ...preferences, search: "changed" }, false, storage), /already exists/);
  assert.deepEqual(readWorkspaces(storage), [{ name: "Payment", preferences }]);
});

test("saving, updating and deleting read current storage and preserve other windows' presets", () => {
  const storage = memoryStorage();
  saveWorkspace("Payment", preferences, false, storage);
  const staleList = readWorkspaces(storage);
  saveWorkspace("Auth", { ...preferences, search: "login" }, false, storage);
  saveWorkspace(staleList[0].name, { ...preferences, search: "retry" }, true, storage);
  assert.equal(readWorkspaces(storage).find((workspace) => workspace.name === "Auth")?.preferences.search, "login");
  assert.equal(readWorkspaces(storage).find((workspace) => workspace.name === "Payment")?.preferences.search, "retry");
  deleteWorkspace("Payment", storage);
  assert.deepEqual(readWorkspaces(storage).map((workspace) => workspace.name), ["Auth"]);
  assert.throws(() => saveWorkspace("Payment", preferences, true, storage), /no longer exists/);
});

test("failed persistence is reported instead of claiming a save or delete succeeded", () => {
  const storage = memoryStorage();
  saveWorkspace("Payment", preferences, false, storage);
  const failingStorage = { ...storage, setItem: () => { throw new Error("Storage full"); } };
  assert.throws(() => saveWorkspace("New", preferences, false, failingStorage), /Storage full/);
  assert.throws(() => saveWorkspace("Payment", { ...preferences, search: "lost" }, true, failingStorage), /Storage full/);
  assert.throws(() => deleteWorkspace("Payment", failingStorage), /Storage full/);
  assert.deepEqual(readWorkspaces(storage), [{ name: "Payment", preferences }]);
});
