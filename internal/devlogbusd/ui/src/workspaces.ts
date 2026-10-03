export type LogLevel = "DEBUG" | "INFO" | "WARN" | "ERROR";
export type ViewMode = "merged" | "source";
export type SourceLayout = "tiled" | "vertical" | "horizontal";

export const levels: LogLevel[] = ["DEBUG", "INFO", "WARN", "ERROR"];
export const workspacesStorageKey = "devlogbus-workspaces";

export type ViewerPreferences = {
  viewMode: ViewMode;
  sourceLayout: SourceLayout;
  paneWidth: number;
  excludedSources: string[];
  blockedSources: string[];
  selectedLevels: LogLevel[];
  perSourceLevels: Partial<Record<string, LogLevel[]>>;
  search: string;
  mergedAutoScroll: boolean;
  mergedLineDetails: boolean;
  autoScrollSources: Partial<Record<string, boolean>>;
  detailSources: Partial<Record<string, boolean>>;
  groupViewModes: Partial<Record<string, ViewMode>>;
  groupLayouts: Partial<Record<string, SourceLayout>>;
  groupPaneWidths: Partial<Record<string, number>>;
};

export type SavedWorkspace = {
  name: string;
  preferences: ViewerPreferences;
};

type WorkspaceStorage = Pick<Storage, "getItem" | "setItem">;

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value != null && !Array.isArray(value);
}

function isStringList(value: unknown): value is string[] {
  return Array.isArray(value) && value.every((item) => typeof item === "string");
}

function isLevelList(value: unknown): value is LogLevel[] {
  return Array.isArray(value) && value.every((item) => levels.includes(item));
}

function isViewMode(value: unknown): value is ViewMode {
  return value === "merged" || value === "source";
}

function isSourceLayout(value: unknown): value is SourceLayout {
  return value === "tiled" || value === "vertical" || value === "horizontal";
}

function isPaneWidth(value: unknown): value is number {
  return typeof value === "number" && Number.isFinite(value) && value > 0;
}

function isSettingMap(value: unknown, valid: (setting: unknown) => boolean): boolean {
  return isObject(value) && Object.values(value).every(valid);
}

function isWorkspace(value: unknown): value is SavedWorkspace {
  if (!isObject(value) || typeof value.name !== "string" || value.name.trim() === "") {
    return false;
  }
  const settings = value.preferences;
  return (
    isObject(settings) &&
    isViewMode(settings.viewMode) &&
    isSourceLayout(settings.sourceLayout) &&
    isPaneWidth(settings.paneWidth) &&
    isStringList(settings.excludedSources) &&
    isStringList(settings.blockedSources) &&
    isLevelList(settings.selectedLevels) &&
    isSettingMap(settings.perSourceLevels, isLevelList) &&
    typeof settings.search === "string" &&
    typeof settings.mergedAutoScroll === "boolean" &&
    typeof settings.mergedLineDetails === "boolean" &&
    isSettingMap(settings.autoScrollSources, (setting) => typeof setting === "boolean") &&
    isSettingMap(settings.detailSources, (setting) => typeof setting === "boolean") &&
    isSettingMap(settings.groupViewModes, isViewMode) &&
    isSettingMap(settings.groupLayouts, isSourceLayout) &&
    isSettingMap(settings.groupPaneWidths, isPaneWidth)
  );
}

export function readWorkspaces(storage?: WorkspaceStorage): SavedWorkspace[] {
  try {
    const saved = (storage ?? window.localStorage).getItem(workspacesStorageKey);
    if (saved == null) {
      return [];
    }
    const parsed: unknown = JSON.parse(saved);
    if (!isObject(parsed) || parsed.version !== 1 || !Array.isArray(parsed.workspaces)) {
      return [];
    }
    const names = new Set<string>();
    return parsed.workspaces.filter(isWorkspace).flatMap((workspace) => {
      const name = workspace.name.trim();
      const key = name.toLowerCase();
      if (names.has(key)) {
        return [];
      }
      names.add(key);
      return [{ ...workspace, name }];
    }).sort((a, b) => a.name.localeCompare(b.name));
  } catch {
    return [];
  }
}

export function saveWorkspace(
  name: string,
  preferences: ViewerPreferences,
  replace = false,
  storage?: WorkspaceStorage,
): SavedWorkspace[] {
  const trimmed = name.trim();
  if (trimmed === "") {
    throw new Error("Enter a workspace name.");
  }
  const workspaces = readWorkspaces(storage);
  const index = workspaces.findIndex(
    (workspace) => workspace.name.toLowerCase() === trimmed.toLowerCase(),
  );
  if (index >= 0 && !replace) {
    throw new Error("A workspace with that name already exists. Use Update to replace it.");
  }
  if (index < 0 && replace) {
    throw new Error("That workspace no longer exists. Save it as a new workspace.");
  }
  const workspace = { name: index >= 0 ? workspaces[index].name : trimmed, preferences };
  if (index >= 0) {
    workspaces[index] = workspace;
  } else {
    workspaces.push(workspace);
  }
  workspaces.sort((a, b) => a.name.localeCompare(b.name));
  (storage ?? window.localStorage).setItem(
    workspacesStorageKey,
    JSON.stringify({ version: 1, workspaces }),
  );
  return workspaces;
}

export function deleteWorkspace(name: string, storage?: WorkspaceStorage): SavedWorkspace[] {
  const workspaces = readWorkspaces(storage).filter((workspace) => workspace.name !== name);
  (storage ?? window.localStorage).setItem(
    workspacesStorageKey,
    JSON.stringify({ version: 1, workspaces }),
  );
  return workspaces;
}
