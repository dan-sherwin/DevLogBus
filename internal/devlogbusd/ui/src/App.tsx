import { type CSSProperties, useEffect, useMemo, useRef, useState } from "react";
import {
  Box,
  Button,
  CssBaseline,
  IconButton,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  Slider,
  Switch,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Tooltip,
  Typography,
} from "@mui/material";
import { createTheme, ThemeProvider } from "@mui/material/styles";
import ClearAllIcon from "@mui/icons-material/ClearAll";
import ComputerIcon from "@mui/icons-material/Computer";
import DarkModeIcon from "@mui/icons-material/DarkMode";
import DeleteSweepIcon from "@mui/icons-material/DeleteSweep";
import KeyboardDoubleArrowDownIcon from "@mui/icons-material/KeyboardDoubleArrowDown";
import LightModeIcon from "@mui/icons-material/LightMode";
import MoreVertIcon from "@mui/icons-material/MoreVert";
import PauseCircleOutlinedIcon from "@mui/icons-material/PauseCircleOutlined";
import SubjectIcon from "@mui/icons-material/Subject";
import { useMessagesContext } from "@dsherwin/mui-kit";

type LogLevel = "DEBUG" | "INFO" | "WARN" | "ERROR";
type ConnectionState = "connecting" | "online" | "reconnecting";
type ViewMode = "merged" | "source";
type SourceLayout = "tiled" | "vertical" | "horizontal";
type ThemePreference = "system" | "light" | "dark";
type ResolvedThemeMode = "light" | "dark";

type PaneArea = {
  height: number;
  width: number;
};

type SourceGroup = {
  childSources: string[];
  group: string;
  records: LogRecord[];
};

type SourcePaneModel = {
  childPanes: ChildSourcePane[];
  childSources: string[];
  group: string;
  isGroup: boolean;
  layout: SourceLayout;
  levels: LogLevel[];
  records: LogRecord[];
  scopeKey: string;
  total: number;
  viewMode: ViewMode;
  visibleRecords: LogRecord[];
};

type ChildSourcePane = {
  levels: LogLevel[];
  records: LogRecord[];
  scopeKey: string;
  source: string;
  total: number;
};

type LogRecord = {
  id: string;
  time: string;
  level: string;
  source: string;
  message: string;
  attrs?: Record<string, unknown>;
};

type ViteImportMeta = ImportMeta & {
  readonly env?: {
    readonly VITE_DEVLOGBUS_API_URL?: string;
  };
};

const apiBase = (
  (import.meta as ViteImportMeta).env?.VITE_DEVLOGBUS_API_URL ?? ""
).replace(/\/$/, "");
const maxVisibleRecordsPerSource = 1000;
const levels: LogLevel[] = ["DEBUG", "INFO", "WARN", "ERROR"];
const layouts: Array<{ label: string; value: SourceLayout }> = [
  { label: "Tiled", value: "tiled" },
  { label: "Vertical", value: "vertical" },
  { label: "Horizontal", value: "horizontal" },
];
const sourcePaneGap = 12;
const paneWidthBounds = { min: 360, step: 20 };
const defaultPaneWidth = 380;
const paneHeightMin = 220;
const themeStorageKey = "devlogbus-theme";
const levelClass: Record<LogLevel, string> = {
  DEBUG: "debug",
  INFO: "info",
  WARN: "warn",
  ERROR: "error",
};
const levelShortLabel: Record<LogLevel, string> = {
  DEBUG: "D",
  INFO: "I",
  WARN: "W",
  ERROR: "E",
};

function normalizeLevel(level: string): LogLevel {
  const upper = level.trim().toUpperCase();
  if (upper === "WARN" || upper === "WARNING") {
    return "WARN";
  }
  if (upper === "ERROR" || upper === "ERR") {
    return "ERROR";
  }
  if (upper === "DEBUG" || upper === "DBG") {
    return "DEBUG";
  }
  return "INFO";
}

function formatTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.valueOf())) {
    return value;
  }
  return `${date.toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  })}.${date.getMilliseconds().toString().padStart(3, "0")}`;
}

function attrText(value: unknown): string {
  if (value == null) {
    return "";
  }
  if (typeof value === "string") {
    return value;
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}

function attrSummary(attrs: Record<string, unknown> | undefined): string {
  return Object.entries(attrs ?? {})
    .map(([key, value]) => `${key}=${attrText(value)}`)
    .join(" ");
}

function searchableText(record: LogRecord): string {
  return [
    record.time,
    record.level,
    record.source,
    record.message,
    ...Object.entries(record.attrs ?? {}).flatMap(([key, value]) => [key, attrText(value)]),
  ]
    .join(" ")
    .toLowerCase();
}

function mergeRecord(records: LogRecord[], record: LogRecord): LogRecord[] {
  const key = record.id || `${record.time}:${record.source}:${record.message}`;
  const next = new Map(records.map((existing) => [existing.id, existing]));
  next.set(key, { ...record, id: key });
  const merged = Array.from(next.values());
  const sourceRecords = merged.filter((existing) => existing.source === record.source);
  let drop = sourceRecords.length - maxVisibleRecordsPerSource;
  if (drop <= 0) {
    return merged;
  }
  return merged.filter((existing) => {
    if (existing.source !== record.source) {
      return true;
    }
    if (drop > 0) {
      drop--;
      return false;
    }
    return true;
  });
}

function toggleLevel(selected: LogLevel[], level: LogLevel): LogLevel[] {
  if (selected.includes(level)) {
    return selected.filter((item) => item !== level);
  }
  return levels.filter((item) => item === level || selected.includes(item));
}

function toggleSource(excluded: string[], source: string): string[] {
  if (excluded.includes(source)) {
    return excluded.filter((item) => item !== source);
  }
  return [...excluded, source].sort();
}

function recordMatchesSearch(record: LogRecord, query: string): boolean {
  return query === "" || searchableText(record).includes(query);
}

function attrString(attrs: Record<string, unknown> | undefined, key: string): string {
  const value = attrs?.[key];
  return typeof value === "string" ? value.trim() : "";
}

function defaultChromeSource(rawURL: string, tabID: unknown): string {
  try {
    const url = new URL(rawURL);
    return `chrome:${url.host}`;
  } catch {
    return `chrome:tab-${typeof tabID === "number" || typeof tabID === "string" ? tabID : "unknown"}`;
  }
}

function recordSourceGroup(record: LogRecord): string {
  const explicit = attrString(record.attrs, "sourceGroup") || attrString(record.attrs, "source_group");
  if (explicit !== "") {
    return explicit;
  }
  if (record.source.startsWith("chrome:")) {
    const tabURL = attrString(record.attrs, "tabURL");
    if (tabURL !== "") {
      return defaultChromeSource(tabURL, record.attrs?.tabId);
    }
  }
  return record.source;
}

function groupKey(group: string): string {
  return `group:${group}`;
}

function sourceKey(source: string): string {
  return `source:${source}`;
}

function buildSourceGroups(records: LogRecord[]): SourceGroup[] {
  const groups = new Map<string, SourceGroup>();
  for (const record of records) {
    const group = recordSourceGroup(record);
    const entry = groups.get(group) ?? {
      childSources: [],
      group,
      records: [],
    };
    entry.records.push(record);
    if (record.source !== "" && !entry.childSources.includes(record.source)) {
      entry.childSources.push(record.source);
    }
    groups.set(group, entry);
  }

  return Array.from(groups.values())
    .map((group) => ({
      ...group,
      childSources: group.childSources.sort(),
      records: group.records.sort(compareRecords),
    }))
    .sort((a, b) => a.group.localeCompare(b.group));
}

function compareRecords(a: LogRecord, b: LogRecord): number {
  return new Date(a.time).valueOf() - new Date(b.time).valueOf();
}

function sourceLevels(
  perSourceLevels: Partial<Record<string, LogLevel[]>>,
  source: string,
): LogLevel[] {
  return perSourceLevels[source] ?? levels;
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

function browserHeight(): number {
  if (typeof window === "undefined") {
    return 720;
  }
  return Math.max(paneHeightMin, Math.floor(window.innerHeight));
}

function browserPaneHeight(sourcePaneArea: HTMLElement | null): number {
  if (typeof window === "undefined") {
    return 720;
  }
  if (sourcePaneArea == null) {
    return browserHeight();
  }
  const shell = sourcePaneArea.closest(".shell");
  const shellPaddingBottom =
    shell instanceof HTMLElement ? Number.parseFloat(getComputedStyle(shell).paddingBottom) : 0;
  const available = window.innerHeight - sourcePaneArea.getBoundingClientRect().top;
  return Math.max(paneHeightMin, Math.floor(available - (shellPaddingBottom || 0)));
}

function browserWidth(): number {
  if (typeof window === "undefined") {
    return 1100;
  }
  return Math.max(paneWidthBounds.min, Math.floor(window.innerWidth));
}

function savedThemePreference(): ThemePreference {
  if (typeof window === "undefined") {
    return "system";
  }
  const saved = window.localStorage.getItem(themeStorageKey);
  if (saved === "system" || saved === "light" || saved === "dark") {
    return saved;
  }
  return "system";
}

function systemThemeMode(): ResolvedThemeMode {
  if (typeof window === "undefined") {
    return "dark";
  }
  return window.matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark";
}

function browserPaneWidth(sourcePaneArea: HTMLElement | null): number {
  if (sourcePaneArea == null) {
    return browserWidth();
  }
  return Math.max(paneWidthBounds.min, Math.floor(sourcePaneArea.getBoundingClientRect().width));
}

function sourceAutoScroll(
  autoScrollSources: Partial<Record<string, boolean>>,
  source: string,
): boolean {
  return autoScrollSources[source] ?? true;
}

function sourceLineDetails(
  detailSources: Partial<Record<string, boolean>>,
  source: string,
): boolean {
  return detailSources[source] ?? false;
}

function sourcePaused(pausedSources: Partial<Record<string, boolean>>, source: string): boolean {
  return pausedSources[source] ?? false;
}

function withoutSourceSetting<T>(
  settings: Partial<Record<string, T>>,
  source: string,
): Partial<Record<string, T>> {
  const next = { ...settings };
  delete next[source];
  return next;
}

export default function App() {
  const { displayErrorMessage, displaySuccessMessage } = useMessagesContext();
  const [themePreference, setThemePreference] = useState<ThemePreference>(savedThemePreference);
  const [systemTheme, setSystemTheme] = useState<ResolvedThemeMode>(systemThemeMode);
  const [records, setRecords] = useState<LogRecord[]>([]);
  const [knownSources, setKnownSources] = useState<string[]>([]);
  const [connection, setConnection] = useState<ConnectionState>("connecting");
  const [paused, setPaused] = useState(false);
  const [search, setSearch] = useState("");
  const [viewMode, setViewMode] = useState<ViewMode>("merged");
  const [sourceLayout, setSourceLayout] = useState<SourceLayout>("tiled");
  const [paneArea, setPaneArea] = useState<PaneArea>(() => ({
    height: browserHeight(),
    width: browserWidth(),
  }));
  const [paneWidth, setPaneWidth] = useState(defaultPaneWidth);
  const [selectedLevels, setSelectedLevels] = useState<LogLevel[]>(levels);
  const [perSourceLevels, setPerSourceLevels] = useState<Partial<Record<string, LogLevel[]>>>({});
  const [autoScrollSources, setAutoScrollSources] = useState<Partial<Record<string, boolean>>>({});
  const [detailSources, setDetailSources] = useState<Partial<Record<string, boolean>>>({});
  const [pausedSources, setPausedSources] = useState<Partial<Record<string, boolean>>>({});
  const [mergedAutoScroll, setMergedAutoScroll] = useState(true);
  const [mergedLineDetails, setMergedLineDetails] = useState(false);
  const [mergedPaneHeight, setMergedPaneHeight] = useState(browserHeight);
  const [excludedSources, setExcludedSources] = useState<string[]>([]);
  const [groupViewModes, setGroupViewModes] = useState<Partial<Record<string, ViewMode>>>({});
  const [groupLayouts, setGroupLayouts] = useState<Partial<Record<string, SourceLayout>>>({});
  const [selectedID, setSelectedID] = useState("");
  const pausedRef = useRef(paused);
  const pausedSourcesRef = useRef(pausedSources);
  const viewModeRef = useRef(viewMode);
  const mergedLogListRef = useRef<HTMLDivElement | null>(null);
  const mergedPaneRef = useRef<HTMLElement | null>(null);
  const sourcePaneAreaRef = useRef<HTMLDivElement | null>(null);
  const paneLogListsRef = useRef<Record<string, HTMLDivElement | undefined>>({});
  const resolvedThemeMode: ResolvedThemeMode =
    themePreference === "system" ? systemTheme : themePreference;
  const appTheme = useMemo(
    () =>
      createTheme({
        palette: {
          mode: resolvedThemeMode,
          primary: {
            main: "#51a7d9",
          },
          background: {
            default: resolvedThemeMode === "dark" ? "#101214" : "#f4f7fa",
            paper: resolvedThemeMode === "dark" ? "#181b1f" : "#ffffff",
          },
          text: {
            primary: resolvedThemeMode === "dark" ? "#eef2f6" : "#15202b",
            secondary: resolvedThemeMode === "dark" ? "#c4ccd4" : "#5d6b78",
          },
        },
        shape: {
          borderRadius: 6,
        },
        typography: {
          fontFamily:
            'Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif',
        },
      }),
    [resolvedThemeMode],
  );

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    const media = window.matchMedia("(prefers-color-scheme: light)");
    const syncSystemTheme = () => setSystemTheme(media.matches ? "light" : "dark");
    syncSystemTheme();
    media.addEventListener("change", syncSystemTheme);
    return () => media.removeEventListener("change", syncSystemTheme);
  }, []);

  useEffect(() => {
    if (typeof document !== "undefined") {
      document.documentElement.dataset.theme = resolvedThemeMode;
      document.documentElement.style.colorScheme = resolvedThemeMode;
    }
    if (typeof window !== "undefined") {
      window.localStorage.setItem(themeStorageKey, themePreference);
    }
  }, [resolvedThemeMode, themePreference]);

  useEffect(() => {
    pausedRef.current = paused;
  }, [paused]);

  useEffect(() => {
    pausedSourcesRef.current = pausedSources;
  }, [pausedSources]);

  useEffect(() => {
    viewModeRef.current = viewMode;
  }, [viewMode]);

  useEffect(() => {
    const params = new URLSearchParams({ level: "debug" });
    const stream = new EventSource(`${apiBase}/api/stream?${params.toString()}`);

    stream.onopen = () => setConnection("online");
    stream.onerror = () => setConnection("reconnecting");
    stream.addEventListener("record", (event) => {
      try {
        const record = JSON.parse((event as MessageEvent<string>).data) as LogRecord;
        if (viewModeRef.current === "merged" && pausedRef.current) {
          return;
        }
        const group = recordSourceGroup(record);
        if (
          sourcePaused(pausedSourcesRef.current, groupKey(group)) ||
          sourcePaused(pausedSourcesRef.current, sourceKey(record.source))
        ) {
          return;
        }
        setKnownSources((current) => {
          if (record.source === "" || current.includes(record.source)) {
            return current;
          }
          return [...current, record.source].sort();
        });
        setRecords((current) => mergeRecord(current, record));
      } catch (error) {
        console.error("Failed to parse DevLogBus record", error);
      }
    });

    return () => stream.close();
  }, []);

  useEffect(() => {
    const syncPaneBounds = () => {
      const nextArea = {
        height: browserPaneHeight(sourcePaneAreaRef.current),
        width: browserPaneWidth(sourcePaneAreaRef.current),
      };
      setPaneArea(nextArea);
      setPaneWidth((current) => clamp(current, paneWidthBounds.min, nextArea.width));
    };

    syncPaneBounds();
    window.addEventListener("resize", syncPaneBounds);
    return () => window.removeEventListener("resize", syncPaneBounds);
  }, [records.length, sourceLayout, viewMode]);

  useEffect(() => {
    const syncMergedPaneHeight = () => {
      setMergedPaneHeight(browserPaneHeight(mergedPaneRef.current));
    };

    syncMergedPaneHeight();
    window.addEventListener("resize", syncMergedPaneHeight);
    return () => window.removeEventListener("resize", syncMergedPaneHeight);
  }, [records.length, viewMode]);

  const sourceGroups = useMemo(() => buildSourceGroups(records), [records]);
  const sources = useMemo(() => sourceGroups.map((group) => group.group), [sourceGroups]);
  const selectedGroups = useMemo(
    () => sourceGroups.filter((group) => !excludedSources.includes(group.group)),
    [excludedSources, sourceGroups],
  );

  const mergedRecords = useMemo(() => {
    const query = search.trim().toLowerCase();
    return records.filter((record) => {
      const normalized = normalizeLevel(record.level);
      if (!selectedLevels.includes(normalized)) {
        return false;
      }
      if (excludedSources.includes(recordSourceGroup(record))) {
        return false;
      }
      if (!recordMatchesSearch(record, query)) {
        return false;
      }
      return true;
    }).sort(compareRecords);
  }, [excludedSources, records, search, selectedLevels]);

  const sourcePanes = useMemo(() => {
    const query = search.trim().toLowerCase();
    return selectedGroups.map((group): SourcePaneModel => {
      const isGroup = group.childSources.length > 1;
      const singleSource = group.childSources[0] ?? group.group;
      const scope = isGroup ? groupKey(group.group) : sourceKey(singleSource);
      const paneLevels = sourceLevels(perSourceLevels, scope);
      const visibleRecords = group.records.filter(
        (record) =>
          paneLevels.includes(normalizeLevel(record.level)) && recordMatchesSearch(record, query),
      );
      const childPanes = group.childSources.map((source): ChildSourcePane => {
        const childScope = sourceKey(source);
        const childLevels = sourceLevels(perSourceLevels, childScope);
        const sourceRecords = group.records.filter((record) => record.source === source);
        return {
          levels: childLevels,
          records: sourceRecords.filter(
            (record) =>
              childLevels.includes(normalizeLevel(record.level)) &&
              recordMatchesSearch(record, query),
          ),
          scopeKey: childScope,
          source,
          total: sourceRecords.length,
        };
      });
      return {
        childPanes,
        childSources: group.childSources,
        group: group.group,
        isGroup,
        layout: groupLayouts[group.group] ?? "tiled",
        levels: paneLevels,
        records: group.records,
        scopeKey: scope,
        total: group.records.length,
        viewMode: groupViewModes[group.group] ?? "merged",
        visibleRecords,
      };
    });
  }, [groupLayouts, groupViewModes, perSourceLevels, search, selectedGroups]);

  const sourceVisibleRecords = useMemo(() => {
    const query = search.trim().toLowerCase();
    return records.filter((record) => {
      const group = recordSourceGroup(record);
      if (excludedSources.includes(group)) {
        return false;
      }
      const sourceGroup = sourceGroups.find((item) => item.group === group);
      const isGroup = (sourceGroup?.childSources.length ?? 0) > 1;
      const groupViewMode = groupViewModes[group] ?? "merged";
      const scope =
        isGroup && groupViewMode === "merged" ? groupKey(group) : sourceKey(record.source);
      if (!sourceLevels(perSourceLevels, scope).includes(normalizeLevel(record.level))) {
        return false;
      }
      return recordMatchesSearch(record, query);
    }).sort(compareRecords);
  }, [excludedSources, groupViewModes, perSourceLevels, records, search, sourceGroups]);

  const visibleRecords = viewMode === "merged" ? mergedRecords : sourceVisibleRecords;
  const displayedCount = visibleRecords.length;
  const selected =
    visibleRecords.find((record) => record.id === selectedID) ?? visibleRecords.at(-1) ?? null;

  const clearRecords = () => {
    setRecords([]);
    setKnownSources([]);
    setPausedSources({});
    setGroupViewModes({});
    setGroupLayouts({});
    setSelectedID("");
  };

  const clearSourceRecords = (source: string, options?: { forgetSource?: boolean }) => {
    setRecords((current) => current.filter((record) => record.source !== source));
    if (options?.forgetSource === true) {
      setKnownSources((current) => current.filter((item) => item !== source));
      const scope = sourceKey(source);
      setExcludedSources((current) => current.filter((item) => item !== source));
      setPerSourceLevels((current) => withoutSourceSetting(current, scope));
      setAutoScrollSources((current) => withoutSourceSetting(current, scope));
      setDetailSources((current) => withoutSourceSetting(current, scope));
      setPausedSources((current) => withoutSourceSetting(current, scope));
    } else {
      setKnownSources((current) => {
        if (current.includes(source)) {
          return current;
        }
        return [...current, source].sort();
      });
    }
    setSelectedID((currentID) => {
      const selectedRecord = records.find((record) => record.id === currentID);
      return selectedRecord?.source === source ? "" : currentID;
    });
  };

  const clearGroupRecords = (group: string, options?: { forgetGroup?: boolean }) => {
    setRecords((current) => current.filter((record) => recordSourceGroup(record) !== group));
    if (options?.forgetGroup === true) {
      const scope = groupKey(group);
      setExcludedSources((current) => current.filter((item) => item !== group));
      setGroupViewModes((current) => withoutSourceSetting(current, group));
      setGroupLayouts((current) => withoutSourceSetting(current, group));
      setPerSourceLevels((current) => withoutSourceSetting(current, scope));
      setAutoScrollSources((current) => withoutSourceSetting(current, scope));
      setDetailSources((current) => withoutSourceSetting(current, scope));
      setPausedSources((current) => withoutSourceSetting(current, scope));
    }
    setSelectedID((currentID) => {
      const selectedRecord = records.find((record) => record.id === currentID);
      return selectedRecord != null && recordSourceGroup(selectedRecord) === group ? "" : currentID;
    });
  };

  const expungeRecords = async (source?: string) => {
    const params = new URLSearchParams();
    if (source != null && source !== "") {
      params.set("source", source);
    }
    const query = params.toString();
    try {
      const response = await fetch(`${apiBase}/api/records/expunge${query ? `?${query}` : ""}`, {
        method: "DELETE",
      });
      if (!response.ok) {
        throw new Error(`expunge failed: ${response.status}`);
      }
      const result = (await response.json()) as { expunged?: number };
      if (source != null && source !== "") {
        clearSourceRecords(source, { forgetSource: true });
        displaySuccessMessage(`Expunged ${result.expunged ?? 0} ${source} records`);
        return;
      }
      clearRecords();
      displaySuccessMessage(`Expunged ${result.expunged ?? 0} records`);
    } catch (error) {
      console.error("Failed to expunge DevLogBus records", error);
      displayErrorMessage("Failed to expunge DevLogBus records");
    }
  };

  const expungeGroupRecords = async (pane: SourcePaneModel) => {
    try {
      let expunged = 0;
      for (const source of pane.childSources) {
        const response = await fetch(
          `${apiBase}/api/records/expunge?${new URLSearchParams({ source }).toString()}`,
          {
            method: "DELETE",
          },
        );
        if (!response.ok) {
          throw new Error(`expunge failed: ${response.status}`);
        }
        const result = (await response.json()) as { expunged?: number };
        expunged += result.expunged ?? 0;
      }
      clearGroupRecords(pane.group, { forgetGroup: true });
      displaySuccessMessage(`Expunged ${expunged} ${pane.group} records`);
    } catch (error) {
      console.error("Failed to expunge DevLogBus group records", error);
      displayErrorMessage("Failed to expunge DevLogBus group records");
    }
  };

  const togglePaneLevel = (source: string, level: LogLevel) => {
    setPerSourceLevels((current) => ({
      ...current,
      [source]: toggleLevel(sourceLevels(current, source), level),
    }));
  };

  const toggleAutoScroll = (source: string, enabled: boolean) => {
    setAutoScrollSources((current) => ({
      ...current,
      [source]: enabled,
    }));
  };

  const toggleLineDetails = (source: string, enabled: boolean) => {
    setDetailSources((current) => ({
      ...current,
      [source]: enabled,
    }));
  };

  const toggleSourcePause = (source: string, enabled: boolean) => {
    setPausedSources((current) => ({
      ...current,
      [source]: enabled,
    }));
  };

  const tileColumnCount = Math.max(
    1,
    Math.floor((paneArea.width + sourcePaneGap) / (paneWidth + sourcePaneGap)),
  );
  const tileRowCount = Math.max(1, Math.ceil(sourcePanes.length / tileColumnCount));
  const tileRowHeight = Math.max(
    0,
    Math.floor((paneArea.height - sourcePaneGap * (tileRowCount - 1)) / tileRowCount),
  );

  useEffect(() => {
    if (viewMode !== "merged" || !mergedAutoScroll) {
      return;
    }
    const frame = window.requestAnimationFrame(() => {
      const list = mergedLogListRef.current;
      if (list == null) {
        return;
      }
      list.scrollTop = list.scrollHeight;
    });
    return () => window.cancelAnimationFrame(frame);
  }, [mergedAutoScroll, mergedLineDetails, mergedRecords, viewMode]);

  useEffect(() => {
    if (viewMode !== "source") {
      return;
    }
    const frame = window.requestAnimationFrame(() => {
      for (const pane of sourcePanes) {
        if (pane.isGroup && pane.viewMode === "source") {
          for (const child of pane.childPanes) {
            if (!sourceAutoScroll(autoScrollSources, child.scopeKey)) {
              continue;
            }
            const list = paneLogListsRef.current[child.scopeKey];
            if (list != null) {
              list.scrollTop = list.scrollHeight;
            }
          }
          continue;
        }
        if (!sourceAutoScroll(autoScrollSources, pane.scopeKey)) {
          continue;
        }
        const list = paneLogListsRef.current[pane.scopeKey];
        if (list != null) {
          list.scrollTop = list.scrollHeight;
        }
      }
    });
    return () => window.cancelAnimationFrame(frame);
  }, [
    autoScrollSources,
    paneArea.height,
    paneWidth,
    sourceLayout,
    sourcePanes,
    tileRowHeight,
    viewMode,
  ]);

  const sourcePaneStyle = {
    "--source-pane-area-height": `${paneArea.height}px`,
    "--source-pane-count": `${Math.max(1, sourcePanes.length)}`,
    "--source-pane-tile-row-height": `${tileRowHeight}px`,
    "--source-pane-width": `${paneWidth}px`,
  } as CSSProperties;
  const mergedPaneStyle = {
    "--merged-pane-height": `${mergedPaneHeight}px`,
  } as CSSProperties;

  return (
    <ThemeProvider theme={appTheme}>
      <CssBaseline />
      <main className="shell" data-theme={resolvedThemeMode}>
        <header className="topbar">
          <div className="brandLockup">
            <img className="brandMark" src="/devlogbus-brand.png" alt="" aria-hidden="true" />
            <div>
              <h1>DevLogBus</h1>
              <p>
                {displayedCount} shown / {records.length} buffered
              </p>
            </div>
          </div>
          <div className="topbarActions">
            <ThemeModeControl onChange={setThemePreference} preference={themePreference} />
            <div className={`status ${connection}`}>
              <span className="dot" />
              broker {connection}
            </div>
          </div>
        </header>

        <section className="toolbar" aria-label="Log filters">
          <TextField
            aria-label="Search logs"
            className="searchInput"
            onChange={(event) => setSearch(event.target.value)}
            placeholder="Search message, source, or field"
            size="small"
            value={search}
            variant="outlined"
          />
          {viewMode === "source" && sourceLayout === "tiled" && (
            <div aria-label="Source pane sizing" className="paneControls">
              <PaneRange
                bounds={{ max: paneArea.width, ...paneWidthBounds }}
                label="Width"
                onChange={(value) =>
                  setPaneWidth(clamp(value, paneWidthBounds.min, paneArea.width))
                }
                value={paneWidth}
              />
            </div>
          )}
          <ToggleButtonGroup
            aria-label="View mode"
            className="modeToggles"
            exclusive
            onChange={(_, value: ViewMode | null) => {
              if (value != null) {
                setViewMode(value);
              }
            }}
            size="small"
            value={viewMode}
          >
            <ToggleButton value="merged">Merged</ToggleButton>
            <ToggleButton value="source">By source</ToggleButton>
          </ToggleButtonGroup>
          {viewMode === "source" && (
            <ToggleButtonGroup
              aria-label="Source layout"
              className="layoutToggles"
              exclusive
              onChange={(_, value: SourceLayout | null) => {
                if (value != null) {
                  setSourceLayout(value);
                }
              }}
              size="small"
              value={sourceLayout}
            >
              {layouts.map((layout) => (
                <ToggleButton key={layout.value} value={layout.value}>
                  {layout.label}
                </ToggleButton>
              ))}
            </ToggleButtonGroup>
          )}
          {sources.length > 0 && (
            <div aria-label="Sources" className="sourceToggles" role="group">
              {sources.map((source) => (
                <Button
                  aria-pressed={!excludedSources.includes(source)}
                  className="sourceToggle"
                  key={source}
                  onClick={() => setExcludedSources((current) => toggleSource(current, source))}
                  size="small"
                  title={source}
                  variant={excludedSources.includes(source) ? "outlined" : "contained"}
                >
                  {source}
                </Button>
              ))}
            </div>
          )}
        </section>

        {viewMode === "merged" ? (
          <section className="content" style={mergedPaneStyle}>
            <section className="mergedPane" ref={mergedPaneRef}>
              <header className="sourcePaneHeader">
                <div className="sourcePaneTitle">
                  <strong>Merged</strong>
                  <span>
                    {mergedRecords.length} / {records.length}
                  </span>
                </div>
                <div className="sourcePaneActions">
                  <LevelButtons
                    ariaLabel="Merged levels"
                    onToggle={(level) =>
                      setSelectedLevels((current) => toggleLevel(current, level))
                    }
                    selected={selectedLevels}
                  />
                  <PaneMenu
                    autoScroll={mergedAutoScroll}
                    details={mergedLineDetails}
                    expungeLabel="Expunge All"
                    label="Merged controls"
                    onAutoScrollChange={setMergedAutoScroll}
                    onClear={clearRecords}
                    onDetailsChange={setMergedLineDetails}
                    onExpunge={() => {
                      void expungeRecords();
                    }}
                    onPauseChange={setPaused}
                    paused={paused}
                  />
                </div>
              </header>
              <div className="logList" ref={mergedLogListRef} aria-label="Live log records">
                {mergedRecords.length === 0 ? (
                  <div className="emptyState">Waiting for records.</div>
                ) : (
                  mergedRecords.map((record) => {
                    const level = normalizeLevel(record.level);
                    const isSelected = selected?.id === record.id;
                    const detailText = mergedLineDetails ? attrSummary(record.attrs) : "";
                    return (
                      <button
                        className={`logRow ${isSelected ? "selected" : ""}`}
                        key={record.id}
                        onClick={() => setSelectedID(record.id)}
                        type="button"
                      >
                        <span className="time">{formatTime(record.time)}</span>
                        <span className={`level ${levelClass[level]}`}>{level}</span>
                        <span className="source">{record.source}</span>
                        <span className="message">
                          <span>{record.message}</span>
                          {detailText !== "" && (
                            <span className="inlineAttrs"> "{detailText}"</span>
                          )}
                        </span>
                      </button>
                    );
                  })
                )}
              </div>
            </section>

            <RecordDetail selected={selected} />
          </section>
        ) : (
          <section className="splitContent">
            <div
              className={`sourcePaneArea ${sourceLayout}`}
              ref={sourcePaneAreaRef}
              style={sourcePaneStyle}
              aria-label="Source log records"
            >
              {sourcePanes.length === 0 ? (
                <div className="emptyState">Waiting for sources.</div>
              ) : (
                sourcePanes.map((pane) => {
                  const showLineDetails = sourceLineDetails(detailSources, pane.scopeKey);
                  const isPaused = sourcePaused(pausedSources, pane.scopeKey);
                  const showRecordSource = pane.isGroup && pane.viewMode === "merged";
                  const visibleCount =
                    pane.isGroup && pane.viewMode === "source"
                      ? pane.childPanes.reduce((total, child) => total + child.records.length, 0)
                      : pane.visibleRecords.length;
                  return (
                    <section
                      className={`sourcePane ${pane.isGroup ? "groupPane" : ""}`}
                      key={pane.group}
                    >
                      <header
                        className={`sourcePaneHeader ${pane.isGroup ? "groupPaneHeader" : ""}`}
                      >
                        <div className="sourcePaneTitle">
                          <strong title={pane.group}>{pane.group}</strong>
                          <span>
                            {visibleCount} / {pane.total}
                          </span>
                          {pane.isGroup && (
                            <span className="sourcePaneBadge">
                              {pane.childSources.length} sources
                            </span>
                          )}
                        </div>
                        <div className="sourcePaneActions">
                          {pane.isGroup && (
                            <ToggleButtonGroup
                              aria-label={`${pane.group} grouping`}
                              className="groupModeToggles"
                              exclusive
                              onChange={(_, value: ViewMode | null) => {
                                if (value != null) {
                                  setGroupViewModes((current) => ({
                                    ...current,
                                    [pane.group]: value,
                                  }));
                                }
                              }}
                              size="small"
                              value={pane.viewMode}
                            >
                              <ToggleButton value="merged">Merged</ToggleButton>
                              <ToggleButton value="source">By source</ToggleButton>
                            </ToggleButtonGroup>
                          )}
                          {pane.isGroup && pane.viewMode === "source" && (
                            <ToggleButtonGroup
                              aria-label={`${pane.group} child source layout`}
                              className="groupLayoutToggles"
                              exclusive
                              onChange={(_, value: SourceLayout | null) => {
                                if (value != null) {
                                  setGroupLayouts((current) => ({
                                    ...current,
                                    [pane.group]: value,
                                  }));
                                }
                              }}
                              size="small"
                              value={pane.layout}
                            >
                              <ToggleButton value="tiled">Tiled</ToggleButton>
                              <ToggleButton value="vertical">Vert</ToggleButton>
                              <ToggleButton value="horizontal">Horiz</ToggleButton>
                            </ToggleButtonGroup>
                          )}
                          {(!pane.isGroup || pane.viewMode === "merged") && (
                            <LevelButtons
                              ariaLabel={`${pane.group} levels`}
                              onToggle={(level) => togglePaneLevel(pane.scopeKey, level)}
                              selected={pane.levels}
                            />
                          )}
                          <PaneMenu
                            autoScroll={sourceAutoScroll(autoScrollSources, pane.scopeKey)}
                            details={showLineDetails}
                            expungeLabel={pane.isGroup ? "Expunge Group" : "Expunge"}
                            label={`${pane.group} controls`}
                            onAutoScrollChange={(enabled) => toggleAutoScroll(pane.scopeKey, enabled)}
                            onClear={() =>
                              pane.isGroup
                                ? clearGroupRecords(pane.group)
                                : clearSourceRecords(pane.childSources[0] ?? pane.group)
                            }
                            onDetailsChange={(enabled) => toggleLineDetails(pane.scopeKey, enabled)}
                            onExpunge={() => {
                              if (pane.isGroup) {
                                void expungeGroupRecords(pane);
                                return;
                              }
                              void expungeRecords(pane.childSources[0] ?? pane.group);
                            }}
                            onPauseChange={(enabled) => toggleSourcePause(pane.scopeKey, enabled)}
                            paused={isPaused}
                          />
                        </div>
                      </header>
                      {pane.isGroup && pane.viewMode === "source" ? (
                        <div
                          className={`nestedSourcePaneArea ${pane.layout}`}
                          style={
                            {
                              "--nested-source-count": pane.childPanes.length,
                            } as CSSProperties
                          }
                        >
                          {pane.childPanes.map((child) => {
                            const childDetails = sourceLineDetails(detailSources, child.scopeKey);
                            const childPaused = sourcePaused(pausedSources, child.scopeKey);
                            return (
                              <section className="nestedSourcePane" key={child.source}>
                                <header className="nestedSourceHeader">
                                  <div className="sourcePaneTitle">
                                    <strong title={child.source}>{child.source}</strong>
                                    <span>
                                      {child.records.length} / {child.total}
                                    </span>
                                  </div>
                                  <div className="sourcePaneActions">
                                    <LevelButtons
                                      ariaLabel={`${child.source} levels`}
                                      onToggle={(level) => togglePaneLevel(child.scopeKey, level)}
                                      selected={child.levels}
                                    />
                                    <PaneMenu
                                      autoScroll={sourceAutoScroll(autoScrollSources, child.scopeKey)}
                                      details={childDetails}
                                      expungeLabel="Expunge"
                                      label={`${child.source} controls`}
                                      onAutoScrollChange={(enabled) =>
                                        toggleAutoScroll(child.scopeKey, enabled)
                                      }
                                      onClear={() => clearSourceRecords(child.source)}
                                      onDetailsChange={(enabled) =>
                                        toggleLineDetails(child.scopeKey, enabled)
                                      }
                                      onExpunge={() => {
                                        void expungeRecords(child.source);
                                      }}
                                      onPauseChange={(enabled) =>
                                        toggleSourcePause(child.scopeKey, enabled)
                                      }
                                      paused={childPaused}
                                    />
                                  </div>
                                </header>
                                <div
                                  className="paneLogList"
                                  ref={(node) => {
                                    if (node == null) {
                                      delete paneLogListsRef.current[child.scopeKey];
                                      return;
                                    }
                                    paneLogListsRef.current[child.scopeKey] = node;
                                  }}
                                >
                                  {child.records.length === 0 ? (
                                    <div className="emptyState">No matching records.</div>
                                  ) : (
                                    child.records.map((record) => {
                                      const level = normalizeLevel(record.level);
                                      const isSelected = selected?.id === record.id;
                                      const detailText = childDetails ? attrSummary(record.attrs) : "";
                                      return (
                                        <button
                                          className={`paneLogRow ${isSelected ? "selected" : ""}`}
                                          key={record.id}
                                          onClick={() => setSelectedID(record.id)}
                                          type="button"
                                        >
                                          <span className="time">{formatTime(record.time)}</span>
                                          <span className={`level ${levelClass[level]}`}>{level}</span>
                                          <span className="message">
                                            <span>{record.message}</span>
                                            {detailText !== "" && (
                                              <span className="inlineAttrs"> "{detailText}"</span>
                                            )}
                                          </span>
                                        </button>
                                      );
                                    })
                                  )}
                                </div>
                              </section>
                            );
                          })}
                        </div>
                      ) : (
                        <div
                          className="paneLogList"
                          ref={(node) => {
                            if (node == null) {
                              delete paneLogListsRef.current[pane.scopeKey];
                              return;
                            }
                            paneLogListsRef.current[pane.scopeKey] = node;
                          }}
                        >
                          {pane.visibleRecords.length === 0 ? (
                            <div className="emptyState">No matching records.</div>
                          ) : (
                            pane.visibleRecords.map((record) => {
                              const level = normalizeLevel(record.level);
                              const isSelected = selected?.id === record.id;
                              const detailText = showLineDetails ? attrSummary(record.attrs) : "";
                              return (
                                <button
                                  className={`paneLogRow ${showRecordSource ? "withSource" : ""} ${isSelected ? "selected" : ""}`}
                                  key={record.id}
                                  onClick={() => setSelectedID(record.id)}
                                  type="button"
                                >
                                  <span className="time">{formatTime(record.time)}</span>
                                  <span className={`level ${levelClass[level]}`}>{level}</span>
                                  {showRecordSource && <span className="source">{record.source}</span>}
                                  <span className="message">
                                    <span>{record.message}</span>
                                    {detailText !== "" && (
                                      <span className="inlineAttrs"> "{detailText}"</span>
                                    )}
                                  </span>
                                </button>
                              );
                            })
                          )}
                        </div>
                      )}
                    </section>
                  );
                })
              )}
            </div>

            <RecordDetail selected={selected} />
          </section>
        )}
      </main>
    </ThemeProvider>
  );
}

function LevelButtons({
  ariaLabel,
  onToggle,
  selected,
}: {
  ariaLabel: string;
  onToggle: (level: LogLevel) => void;
  selected: LogLevel[];
}) {
  return (
    <Box aria-label={ariaLabel} className="paneLevels" role="group">
      {levels.map((level) => (
        <ToggleButton
          aria-label={`${ariaLabel} ${level}`}
          className={`levelToggle ${levelClass[level]}`}
          key={level}
          onClick={() => onToggle(level)}
          selected={selected.includes(level)}
          title={level}
          value={level}
        >
          {levelShortLabel[level]}
        </ToggleButton>
      ))}
    </Box>
  );
}

function ThemeModeControl({
  onChange,
  preference,
}: {
  onChange: (preference: ThemePreference) => void;
  preference: ThemePreference;
}) {
  return (
    <ToggleButtonGroup
      aria-label="Theme mode"
      className="themeToggles"
      exclusive
      onChange={(_, value: ThemePreference | null) => {
        if (value != null) {
          onChange(value);
        }
      }}
      size="small"
      value={preference}
    >
      <ToggleButton aria-label="Follow system theme" title="Follow system" value="system">
        <ComputerIcon fontSize="small" />
      </ToggleButton>
      <ToggleButton aria-label="Light theme" title="Light" value="light">
        <LightModeIcon fontSize="small" />
      </ToggleButton>
      <ToggleButton aria-label="Dark theme" title="Dark" value="dark">
        <DarkModeIcon fontSize="small" />
      </ToggleButton>
    </ToggleButtonGroup>
  );
}

function PaneMenu({
  autoScroll,
  details,
  expungeLabel,
  label,
  onAutoScrollChange,
  onClear,
  onDetailsChange,
  onExpunge,
  onPauseChange,
  paused,
}: {
  autoScroll: boolean;
  details: boolean;
  expungeLabel: string;
  label: string;
  onAutoScrollChange: (enabled: boolean) => void;
  onClear: () => void;
  onDetailsChange: (enabled: boolean) => void;
  onExpunge: () => void;
  onPauseChange: (enabled: boolean) => void;
  paused: boolean;
}) {
  const [anchorEl, setAnchorEl] = useState<HTMLElement | null>(null);
  const open = anchorEl != null;
  const closeMenu = () => setAnchorEl(null);

  return (
    <>
      <Tooltip title={label}>
        <IconButton
          aria-expanded={open}
          aria-label={label}
          className="paneMenuButton"
          onClick={(event) => setAnchorEl(event.currentTarget)}
          size="small"
        >
          <MoreVertIcon fontSize="inherit" />
        </IconButton>
      </Tooltip>
      <Menu
        anchorEl={anchorEl}
        anchorOrigin={{ horizontal: "right", vertical: "bottom" }}
        className="paneMenu"
        onClose={closeMenu}
        open={open}
        transformOrigin={{ horizontal: "right", vertical: "top" }}
      >
        <MenuItem className="paneMenuItem" onClick={() => onPauseChange(!paused)}>
          <ListItemIcon>
            <PauseCircleOutlinedIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>Pause</ListItemText>
          <Switch
            checked={paused}
            onChange={(event) => onPauseChange(event.currentTarget.checked)}
            onClick={(event) => event.stopPropagation()}
            size="small"
          />
        </MenuItem>
        <MenuItem className="paneMenuItem" onClick={() => onAutoScrollChange(!autoScroll)}>
          <ListItemIcon>
            <KeyboardDoubleArrowDownIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>Bottom</ListItemText>
          <Switch
            checked={autoScroll}
            onChange={(event) => onAutoScrollChange(event.currentTarget.checked)}
            onClick={(event) => event.stopPropagation()}
            size="small"
          />
        </MenuItem>
        <MenuItem className="paneMenuItem" onClick={() => onDetailsChange(!details)}>
          <ListItemIcon>
            <SubjectIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>Details</ListItemText>
          <Switch
            checked={details}
            onChange={(event) => onDetailsChange(event.currentTarget.checked)}
            onClick={(event) => event.stopPropagation()}
            size="small"
          />
        </MenuItem>
        <MenuItem
          className="paneMenuItem"
          onClick={() => {
            onClear();
            closeMenu();
          }}
        >
          <ListItemIcon>
            <ClearAllIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>Clear</ListItemText>
        </MenuItem>
        <MenuItem
          className="paneMenuItem destructive"
          onClick={() => {
            onExpunge();
            closeMenu();
          }}
        >
          <ListItemIcon>
            <DeleteSweepIcon color="error" fontSize="small" />
          </ListItemIcon>
          <ListItemText>{expungeLabel}</ListItemText>
        </MenuItem>
      </Menu>
    </>
  );
}

function PaneRange({
  bounds,
  label,
  onChange,
  value,
}: {
  bounds: { max: number; min: number; step: number };
  label: string;
  onChange: (value: number) => void;
  value: number;
}) {
  return (
    <Box className="rangeControl">
      <Typography component="span" variant="caption">
        {label}
      </Typography>
      <Slider
        aria-label={label}
        max={bounds.max}
        min={bounds.min}
        onChange={(_, nextValue) => {
          if (typeof nextValue === "number") {
            onChange(nextValue);
          }
        }}
        size="small"
        step={bounds.step}
        value={value}
      />
      <Typography component="output" variant="caption">
        {value}px
      </Typography>
    </Box>
  );
}

function RecordDetail({ selected }: { selected: LogRecord | null }) {
  return (
    <aside className="detail" aria-label="Selected record fields">
      {selected == null ? (
        <div className="emptyState">No record selected.</div>
      ) : (
        <>
          <div className="detailHeader">
            <span className={`level ${levelClass[normalizeLevel(selected.level)]}`}>
              {normalizeLevel(selected.level)}
            </span>
            <strong>{selected.message}</strong>
          </div>
          <dl>
            <div>
              <dt>id</dt>
              <dd>{selected.id}</dd>
            </div>
            <div>
              <dt>time</dt>
              <dd>{new Date(selected.time).toLocaleString()}</dd>
            </div>
            <div>
              <dt>source</dt>
              <dd>{selected.source}</dd>
            </div>
            <div>
              <dt>sourceGroup</dt>
              <dd>{recordSourceGroup(selected)}</dd>
            </div>
            {Object.entries(selected.attrs ?? {}).map(([key, value]) => (
              <div key={key}>
                <dt>{key}</dt>
                <dd>{attrText(value)}</dd>
              </div>
            ))}
          </dl>
        </>
      )}
    </aside>
  );
}
