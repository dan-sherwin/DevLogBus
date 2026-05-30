import { type CSSProperties, useEffect, useMemo, useRef, useState } from "react";
import {
  Badge,
  Box,
  Button,
  CssBaseline,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
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
import BlockIcon from "@mui/icons-material/Block";
import ClearAllIcon from "@mui/icons-material/ClearAll";
import ComputerIcon from "@mui/icons-material/Computer";
import DarkModeIcon from "@mui/icons-material/DarkMode";
import DeleteSweepIcon from "@mui/icons-material/DeleteSweep";
import HelpIcon from "@mui/icons-material/Help";
import InfoOutlinedIcon from "@mui/icons-material/InfoOutlined";
import KeyboardDoubleArrowDownIcon from "@mui/icons-material/KeyboardDoubleArrowDown";
import LightModeIcon from "@mui/icons-material/LightMode";
import MoreVertIcon from "@mui/icons-material/MoreVert";
import OpenInNewIcon from "@mui/icons-material/OpenInNew";
import PauseCircleOutlinedIcon from "@mui/icons-material/PauseCircleOutlined";
import SubjectIcon from "@mui/icons-material/Subject";
import { useMessagesContext } from "@dsherwin/mui-kit";

type LogLevel = "DEBUG" | "INFO" | "WARN" | "ERROR";
type ConnectionState = "connecting" | "online" | "reconnecting";
type ViewMode = "merged" | "source";
type SourceLayout = "tiled" | "vertical" | "horizontal";
type ThemePreference = "system" | "light" | "dark";
type ResolvedThemeMode = "light" | "dark";
type PopoutKind = "group" | "source";
type ActiveDialog = "about" | "blocked" | "help" | null;

type AboutResponse = {
  api: {
    ok: boolean;
  };
  broker: {
    echo: boolean;
    endpoint: string;
    httpListenAddress: string;
    maxRecords: number;
    tcpListenAddress: string;
  };
  build: {
    appName: string;
    buildDate: string;
    commit: string;
    goVersion?: string;
    modulePath?: string;
    moduleVersion?: string;
    version: string;
  };
};

type PopoutTarget = {
  key: string;
  kind: PopoutKind;
};

type PaneArea = {
  height: number;
  width: number;
};

type SourceGroup = {
  childSources: string[];
  group: string;
  label: string;
  records: LogRecord[];
};

type SourcePaneModel = {
  childPanes: ChildSourcePane[];
  childSourceToggles: ChildSourceToggle[];
  childSources: string[];
  group: string;
  hiddenChildSources: string[];
  isGroup: boolean;
  label: string;
  layout: SourceLayout;
  levels: LogLevel[];
  records: LogRecord[];
  scopeKey: string;
  total: number;
  visibleChildSources: string[];
  viewMode: ViewMode;
  visibleRecords: LogRecord[];
};

type ChildSourceToggle = {
  hidden: boolean;
  label: string;
  source: string;
  total: number;
};

type ChildSourcePane = {
  label: string;
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
const blockedSourcesStorageKey = "devlogbus-blocked-sources";
const blockedSourcesChannelName = "devlogbus-blocked-sources";
const detachedTargetsStorageKey = "devlogbus-detached-targets";
const detachedTargetsChannelName = "devlogbus-detached-targets";
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

function toggleExcludedKey(excluded: string[], key: string): string[] {
  if (excluded.includes(key)) {
    return excluded.filter((item) => item !== key);
  }
  return [...excluded, key].sort();
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

function chromeSourceLabel(source: string, records: LogRecord[]): string {
  if (!source.startsWith("chrome:")) {
    return source;
  }
  const host = source.slice("chrome:".length);
  if (host === "") {
    return source;
  }
  for (const record of records) {
    const tabURL = attrString(record.attrs, "tabURL") || attrString(record.attrs, "url");
    if (tabURL === "" || defaultChromeSource(tabURL, record.attrs?.tabId) !== source) {
      continue;
    }
    const title = (attrString(record.attrs, "tabTitle") || attrString(record.attrs, "title")).replace(
      /\s+/g,
      " ",
    );
    if (title !== "" && title !== host) {
      return `chrome:${title} (${host})`;
    }
  }
  return source;
}

function recordSourceLabel(record: LogRecord): string {
  return chromeSourceLabel(record.source, [record]);
}

function groupKey(group: string): string {
  return `group:${group}`;
}

function sourceKey(source: string): string {
  return `source:${source}`;
}

function isGroupExcluded(excluded: string[], group: string): boolean {
  return excluded.includes(groupKey(group)) || excluded.includes(group);
}

function isSourceExcluded(excluded: string[], source: string): boolean {
  return excluded.includes(sourceKey(source));
}

function targetID(target: PopoutTarget): string {
  return `${target.kind}:${target.key}`;
}

function parseTargetID(value: string | null): PopoutTarget | null {
  if (value == null) {
    return null;
  }
  const delimiter = value.indexOf(":");
  if (delimiter <= 0) {
    return null;
  }
  const kind = value.slice(0, delimiter);
  if (kind !== "group" && kind !== "source") {
    return null;
  }
  const key = value.slice(delimiter + 1);
  if (key === "") {
    return null;
  }
  return { key, kind };
}

function parsePopoutTarget(): PopoutTarget | null {
  if (typeof window === "undefined") {
    return null;
  }
  return parseTargetID(new URLSearchParams(window.location.search).get("popout"));
}

function normalizeTargetIDs(ids: string[]): string[] {
  const seen = new Set<string>();
  const normalized: string[] = [];
  for (const id of ids) {
    const target = parseTargetID(id);
    if (target == null) {
      continue;
    }
    const normalizedID = targetID(target);
    if (seen.has(normalizedID)) {
      continue;
    }
    seen.add(normalizedID);
    normalized.push(normalizedID);
  }
  return normalized.sort();
}

function normalizeSourceNames(sources: string[]): string[] {
  const seen = new Set<string>();
  const normalized: string[] = [];
  for (const source of sources) {
    const trimmed = source.trim();
    if (trimmed === "" || seen.has(trimmed)) {
      continue;
    }
    seen.add(trimmed);
    normalized.push(trimmed);
  }
  return normalized.sort();
}

function readBlockedSources(): string[] {
  if (typeof window === "undefined") {
    return [];
  }
  try {
    const saved = window.localStorage.getItem(blockedSourcesStorageKey);
    if (saved == null || saved === "") {
      return [];
    }
    const parsed = JSON.parse(saved) as unknown;
    return Array.isArray(parsed)
      ? normalizeSourceNames(parsed.filter((item): item is string => typeof item === "string"))
      : [];
  } catch {
    return [];
  }
}

function writeBlockedSources(sources: string[]) {
  if (typeof window === "undefined") {
    return;
  }
  try {
    const normalized = normalizeSourceNames(sources);
    if (normalized.length === 0) {
      window.localStorage.removeItem(blockedSourcesStorageKey);
      return;
    }
    window.localStorage.setItem(blockedSourcesStorageKey, JSON.stringify(normalized));
  } catch {
    // Blocking is a viewer preference; live records continue to render if storage is unavailable.
  }
}

function isSourceBlocked(blockedSources: string[], source: string): boolean {
  return blockedSources.includes(source);
}

function readDetachedTargetIDs(): string[] {
  if (typeof window === "undefined") {
    return [];
  }
  try {
    const saved = window.localStorage.getItem(detachedTargetsStorageKey);
    if (saved == null || saved === "") {
      return [];
    }
    const parsed = JSON.parse(saved) as unknown;
    return Array.isArray(parsed)
      ? normalizeTargetIDs(parsed.filter((item): item is string => typeof item === "string"))
      : [];
  } catch {
    return [];
  }
}

function writeDetachedTargetIDs(ids: string[]) {
  if (typeof window === "undefined") {
    return;
  }
  try {
    const normalized = normalizeTargetIDs(ids);
    if (normalized.length === 0) {
      window.localStorage.removeItem(detachedTargetsStorageKey);
      return;
    }
    window.localStorage.setItem(detachedTargetsStorageKey, JSON.stringify(normalized));
  } catch {
    // Detached-window state is a convenience layer; the live stream still works without storage.
  }
}

function sameTarget(a: PopoutTarget | null, b: PopoutTarget | null): boolean {
  return a != null && b != null && a.kind === b.kind && a.key === b.key;
}

function isTargetDetached(ids: string[], target: PopoutTarget): boolean {
  return ids.includes(targetID(target));
}

function withDetachedTarget(ids: string[], target: PopoutTarget): string[] {
  return normalizeTargetIDs([...ids, targetID(target)]);
}

function withoutDetachedTarget(ids: string[], target: PopoutTarget): string[] {
  const removeID = targetID(target);
  return normalizeTargetIDs(ids.filter((id) => id !== removeID));
}

function popoutWindowName(target: PopoutTarget): string {
  return `devlogbus-${targetID(target).replace(/[^a-z0-9_-]/gi, "-")}`;
}

function detachedTargetLabel(
  target: PopoutTarget,
  sourceGroups: SourceGroup[],
  records: LogRecord[],
): string {
  if (target.kind === "group") {
    return sourceGroups.find((group) => group.group === target.key)?.label ?? target.key;
  }
  return chromeSourceLabel(
    target.key,
    records.filter((record) => record.source === target.key),
  );
}

function buildSourceGroups(records: LogRecord[]): SourceGroup[] {
  const groups = new Map<string, SourceGroup>();
  for (const record of records) {
    const group = recordSourceGroup(record);
    const entry = groups.get(group) ?? {
      childSources: [],
      group,
      label: group,
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
      label: chromeSourceLabel(group.group, group.records),
      records: group.records.sort(compareRecords),
    }))
    .sort((a, b) => a.group.localeCompare(b.group));
}

function buildSourcePopoutGroups(records: LogRecord[], source: string): SourceGroup[] {
  return [
    {
      childSources: [source],
      group: source,
      label: chromeSourceLabel(source, records),
      records: [...records].sort(compareRecords),
    },
  ];
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
  const [popoutTarget] = useState<PopoutTarget | null>(parsePopoutTarget);
  const [activeDialog, setActiveDialog] = useState<ActiveDialog>(null);
  const [about, setAbout] = useState<AboutResponse | null>(null);
  const [aboutError, setAboutError] = useState("");
  const [themePreference, setThemePreference] = useState<ThemePreference>(savedThemePreference);
  const [systemTheme, setSystemTheme] = useState<ResolvedThemeMode>(systemThemeMode);
  const [records, setRecords] = useState<LogRecord[]>([]);
  const [knownSources, setKnownSources] = useState<string[]>([]);
  const [blockedSources, setBlockedSources] = useState<string[]>(readBlockedSources);
  const [connection, setConnection] = useState<ConnectionState>("connecting");
  const [paused, setPaused] = useState(false);
  const [search, setSearch] = useState("");
  const [viewMode, setViewMode] = useState<ViewMode>(() =>
    popoutTarget == null ? "merged" : "source",
  );
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
  const [detachedTargets, setDetachedTargets] = useState<string[]>(readDetachedTargetIDs);
  const [selectedID, setSelectedID] = useState("");
  const blockedSourcesRef = useRef(blockedSources);
  const pausedRef = useRef(paused);
  const pausedSourcesRef = useRef(pausedSources);
  const viewModeRef = useRef(viewMode);
  const popoutWasDetachedRef = useRef(false);
  const blockedSourcesChannelRef = useRef<BroadcastChannel | null>(null);
  const detachedChannelRef = useRef<BroadcastChannel | null>(null);
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
  const isPopout = popoutTarget != null;
  const updateDetachedTargets = (nextTargets: string[]) => {
    const normalized = normalizeTargetIDs(nextTargets);
    writeDetachedTargetIDs(normalized);
    setDetachedTargets(normalized);
    detachedChannelRef.current?.postMessage(normalized);
  };
  const updateBlockedSources = (nextSources: string[]) => {
    const normalized = normalizeSourceNames(nextSources);
    writeBlockedSources(normalized);
    setBlockedSources(normalized);
    blockedSourcesChannelRef.current?.postMessage(normalized);
  };

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
    if (typeof window === "undefined") {
      return;
    }

    const syncTargets = (targets: unknown) => {
      if (!Array.isArray(targets)) {
        return;
      }
      setDetachedTargets(
        normalizeTargetIDs(targets.filter((item): item is string => typeof item === "string")),
      );
    };
    const syncFromStorage = (event: StorageEvent) => {
      if (event.key != null && event.key !== detachedTargetsStorageKey) {
        return;
      }
      setDetachedTargets(readDetachedTargetIDs());
    };

    window.addEventListener("storage", syncFromStorage);
    if (typeof BroadcastChannel !== "undefined") {
      const channel = new BroadcastChannel(detachedTargetsChannelName);
      detachedChannelRef.current = channel;
      channel.onmessage = (event: MessageEvent<unknown>) => syncTargets(event.data);
    }

    return () => {
      window.removeEventListener("storage", syncFromStorage);
      detachedChannelRef.current?.close();
      detachedChannelRef.current = null;
    };
  }, []);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }

    const syncSources = (sources: unknown) => {
      if (!Array.isArray(sources)) {
        return;
      }
      setBlockedSources(
        normalizeSourceNames(sources.filter((item): item is string => typeof item === "string")),
      );
    };
    const syncFromStorage = (event: StorageEvent) => {
      if (event.key != null && event.key !== blockedSourcesStorageKey) {
        return;
      }
      setBlockedSources(readBlockedSources());
    };

    window.addEventListener("storage", syncFromStorage);
    if (typeof BroadcastChannel !== "undefined") {
      const channel = new BroadcastChannel(blockedSourcesChannelName);
      blockedSourcesChannelRef.current = channel;
      channel.onmessage = (event: MessageEvent<unknown>) => syncSources(event.data);
    }

    return () => {
      window.removeEventListener("storage", syncFromStorage);
      blockedSourcesChannelRef.current?.close();
      blockedSourcesChannelRef.current = null;
    };
  }, []);

  useEffect(() => {
    if (popoutTarget == null || typeof window === "undefined") {
      return;
    }
    if (isTargetDetached(detachedTargets, popoutTarget)) {
      popoutWasDetachedRef.current = true;
      return;
    }
    if (popoutWasDetachedRef.current) {
      window.setTimeout(() => window.close(), 80);
    }
  }, [detachedTargets, popoutTarget]);

  useEffect(() => {
    blockedSourcesRef.current = blockedSources;
    setRecords((current) =>
      current.filter((record) => !isSourceBlocked(blockedSources, record.source)),
    );
    setKnownSources((current) =>
      current.filter((source) => !isSourceBlocked(blockedSources, source)),
    );
  }, [blockedSources]);

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
    if (activeDialog !== "about") {
      return;
    }
    let ignore = false;
    setAboutError("");
    fetch(`${apiBase}/api/about`)
      .then((response) => {
        if (!response.ok) {
          throw new Error(`about failed: ${response.status}`);
        }
        return response.json() as Promise<AboutResponse>;
      })
      .then((nextAbout) => {
        if (!ignore) {
          setAbout(nextAbout);
        }
      })
      .catch((error) => {
        console.error("Failed to load DevLogBus about data", error);
        if (!ignore) {
          setAbout(null);
          setAboutError("About data unavailable");
        }
      });
    return () => {
      ignore = true;
    };
  }, [activeDialog]);

  useEffect(() => {
    const params = new URLSearchParams({ level: "debug" });
    const stream = new EventSource(`${apiBase}/api/stream?${params.toString()}`);

    stream.onopen = () => setConnection("online");
    stream.onerror = () => setConnection("reconnecting");
    stream.addEventListener("record", (event) => {
      try {
        const record = JSON.parse((event as MessageEvent<string>).data) as LogRecord;
        if (isSourceBlocked(blockedSourcesRef.current, record.source)) {
          return;
        }
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

  const viewerRecords = useMemo(
    () => records.filter((record) => !isSourceBlocked(blockedSources, record.source)),
    [blockedSources, records],
  );
  const allSourceGroups = useMemo(() => buildSourceGroups(viewerRecords), [viewerRecords]);
  const scopedRecords = useMemo(
    () =>
      viewerRecords.filter((record) => {
        const group = recordSourceGroup(record);
        if (popoutTarget?.kind === "source") {
          return record.source === popoutTarget.key;
        }
        if (popoutTarget?.kind === "group") {
          return (
            group === popoutTarget.key &&
            !isTargetDetached(detachedTargets, { key: record.source, kind: "source" })
          );
        }
        return (
          !isTargetDetached(detachedTargets, { key: group, kind: "group" }) &&
          !isTargetDetached(detachedTargets, { key: record.source, kind: "source" })
        );
      }),
    [detachedTargets, popoutTarget, viewerRecords],
  );
  const sourceGroups = useMemo(() => {
    if (popoutTarget?.kind === "source") {
      return buildSourcePopoutGroups(scopedRecords, popoutTarget.key);
    }
    return buildSourceGroups(scopedRecords);
  }, [popoutTarget, scopedRecords]);
  const selectedGroups = useMemo(
    () => sourceGroups.filter((group) => !isGroupExcluded(excludedSources, group.group)),
    [excludedSources, sourceGroups],
  );
  const detachedTargetChips = useMemo(
    () =>
      detachedTargets
        .map(parseTargetID)
        .filter((target): target is PopoutTarget => target != null)
        .filter((target) => !sameTarget(target, popoutTarget))
        .map((target) => ({
          id: targetID(target),
          label: detachedTargetLabel(target, allSourceGroups, viewerRecords),
          target,
        })),
    [allSourceGroups, detachedTargets, popoutTarget, viewerRecords],
  );
  const popoutLabel = useMemo(
    () =>
      popoutTarget == null
        ? ""
        : detachedTargetLabel(popoutTarget, allSourceGroups, viewerRecords),
    [allSourceGroups, popoutTarget, viewerRecords],
  );
  const projectedGroupKey = (record: LogRecord): string =>
    popoutTarget?.kind === "source" ? popoutTarget.key : recordSourceGroup(record);

  const mergedRecords = useMemo(() => {
    const query = search.trim().toLowerCase();
    return scopedRecords.filter((record) => {
      const normalized = normalizeLevel(record.level);
      if (!selectedLevels.includes(normalized)) {
        return false;
      }
      if (
        isGroupExcluded(excludedSources, projectedGroupKey(record)) ||
        isSourceExcluded(excludedSources, record.source)
      ) {
        return false;
      }
      if (!recordMatchesSearch(record, query)) {
        return false;
      }
      return true;
    }).sort(compareRecords);
  }, [excludedSources, popoutTarget, scopedRecords, search, selectedLevels]);

  const sourcePanes = useMemo(() => {
    const query = search.trim().toLowerCase();
    return selectedGroups.map((group): SourcePaneModel => {
      const isGroup = group.childSources.length > 1;
      const childModels = group.childSources.map((source): ChildSourceToggle => {
        const sourceRecords = group.records.filter((record) => record.source === source);
        return {
          hidden: isSourceExcluded(excludedSources, source),
          label: chromeSourceLabel(source, sourceRecords),
          source,
          total: sourceRecords.length,
        };
      });
      const visibleChildSources = childModels
        .filter((child) => !child.hidden)
        .map((child) => child.source);
      const hiddenChildSources = childModels
        .filter((child) => child.hidden)
        .map((child) => child.source);
      const visibleSourceSet = new Set(visibleChildSources);
      const singleSource = visibleChildSources[0] ?? group.childSources[0] ?? group.group;
      const scope = isGroup ? groupKey(group.group) : sourceKey(singleSource);
      const paneLevels = sourceLevels(perSourceLevels, scope);
      const visibleGroupRecords = group.records.filter((record) =>
        visibleSourceSet.has(record.source),
      );
      const visibleRecords = visibleGroupRecords.filter(
        (record) =>
          paneLevels.includes(normalizeLevel(record.level)) && recordMatchesSearch(record, query),
      );
      const childPanes = visibleChildSources.map((source): ChildSourcePane => {
        const childScope = sourceKey(source);
        const childLevels = sourceLevels(perSourceLevels, childScope);
        const sourceRecords = group.records.filter((record) => record.source === source);
        return {
          label: chromeSourceLabel(source, sourceRecords),
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
        childSourceToggles: childModels,
        childSources: group.childSources,
        group: group.group,
        hiddenChildSources,
        isGroup,
        label: group.label,
        layout: groupLayouts[group.group] ?? "tiled",
        levels: paneLevels,
        records: group.records,
        scopeKey: scope,
        total: group.records.length,
        visibleChildSources,
        viewMode: groupViewModes[group.group] ?? "merged",
        visibleRecords,
      };
    });
  }, [excludedSources, groupLayouts, groupViewModes, perSourceLevels, search, selectedGroups]);

  const sourceVisibleRecords = useMemo(() => {
    const query = search.trim().toLowerCase();
    return scopedRecords.filter((record) => {
      const group = projectedGroupKey(record);
      if (
        isGroupExcluded(excludedSources, group) ||
        isSourceExcluded(excludedSources, record.source)
      ) {
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
  }, [
    excludedSources,
    groupViewModes,
    perSourceLevels,
    popoutTarget,
    scopedRecords,
    search,
    sourceGroups,
  ]);

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
      setExcludedSources((current) =>
        current.filter((item) => item !== source && item !== sourceKey(source)),
      );
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
      setExcludedSources((current) =>
        current.filter((item) => item !== group && item !== groupKey(group)),
      );
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

  const openPopout = (target: PopoutTarget, label: string) => {
    if (typeof window === "undefined") {
      return;
    }
    const url = new URL(window.location.href);
    url.searchParams.set("popout", targetID(target));
    const popup = window.open(
      url.toString(),
      popoutWindowName(target),
      "popup,width=1200,height=760",
    );
    if (popup == null) {
      displayErrorMessage("Popup blocked. Allow popups for DevLogBus and try again.");
      return;
    }
    updateDetachedTargets(withDetachedTarget(detachedTargets, target));
    popup.focus();
    displaySuccessMessage(`Popped out ${label}`);
  };

  const reattachTarget = (target: PopoutTarget) => {
    updateDetachedTargets(withoutDetachedTarget(detachedTargets, target));
    displaySuccessMessage(`Reattached ${detachedTargetLabel(target, allSourceGroups, viewerRecords)}`);
    if (sameTarget(target, popoutTarget) && typeof window !== "undefined") {
      window.setTimeout(() => window.close(), 80);
    }
  };

  const blockSource = (source: string) => {
    const trimmed = source.trim();
    if (trimmed === "") {
      return;
    }
    updateBlockedSources([...blockedSources, trimmed]);
    setRecords((current) => current.filter((record) => record.source !== trimmed));
    setKnownSources((current) => current.filter((item) => item !== trimmed));
    setExcludedSources((current) =>
      current.filter((item) => item !== trimmed && item !== sourceKey(trimmed)),
    );
    setPerSourceLevels((current) => withoutSourceSetting(current, sourceKey(trimmed)));
    setAutoScrollSources((current) => withoutSourceSetting(current, sourceKey(trimmed)));
    setDetailSources((current) => withoutSourceSetting(current, sourceKey(trimmed)));
    setPausedSources((current) => withoutSourceSetting(current, sourceKey(trimmed)));
    setSelectedID((currentID) => {
      const selectedRecord = records.find((record) => record.id === currentID);
      return selectedRecord?.source === trimmed ? "" : currentID;
    });
    displaySuccessMessage(`Blocked ${trimmed}`);
  };

  const unblockSource = (source: string) => {
    updateBlockedSources(blockedSources.filter((item) => item !== source));
    displaySuccessMessage(`Unblocked ${source}`);
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
      <main className={`shell ${isPopout ? "popoutShell" : ""}`} data-theme={resolvedThemeMode}>
        <header className="topbar">
          <div className="brandLockup">
            <img className="brandMark" src="/devlogbus-brand.png" alt="" aria-hidden="true" />
            <div>
              <h1>DevLogBus</h1>
              <p>
                {displayedCount} shown / {scopedRecords.length} buffered
              </p>
            </div>
          </div>
          <div className="topbarActions">
            {popoutTarget != null && (
              <div className="popoutContext">
                <span title={targetID(popoutTarget)}>Detached: {popoutLabel}</span>
                <Button
                  className="reattachButton"
                  onClick={() => reattachTarget(popoutTarget)}
                  size="small"
                  variant="outlined"
                >
                  Reattach
                </Button>
              </div>
            )}
            <ThemeModeControl onChange={setThemePreference} preference={themePreference} />
            <Tooltip title="Blocked sources">
              <IconButton
                aria-label="Blocked sources"
                className="topbarIconButton"
                onClick={() => setActiveDialog("blocked")}
                size="small"
              >
                <Badge
                  badgeContent={blockedSources.length}
                  color="warning"
                  invisible={blockedSources.length === 0}
                >
                  <BlockIcon fontSize="small" />
                </Badge>
              </IconButton>
            </Tooltip>
            <Tooltip title="Help">
              <IconButton
                aria-label="Help"
                className="topbarIconButton"
                onClick={() => setActiveDialog("help")}
                size="small"
              >
                <HelpIcon fontSize="small" />
              </IconButton>
            </Tooltip>
            <Tooltip title="About">
              <IconButton
                aria-label="About"
                className="topbarIconButton"
                onClick={() => setActiveDialog("about")}
                size="small"
              >
                <InfoOutlinedIcon fontSize="small" />
              </IconButton>
            </Tooltip>
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
          {sourceGroups.length > 0 && (
            <div aria-label="Sources" className="sourceToggles" role="group">
              {sourceGroups.map((group) => {
                const source = group.group;
                const hidden = isGroupExcluded(excludedSources, source);
                return (
                  <Button
                    aria-pressed={!hidden}
                    className="sourceToggle"
                    key={source}
                    onClick={() =>
                      setExcludedSources((current) =>
                        toggleExcludedKey(current, groupKey(source)),
                      )
                    }
                    size="small"
                    title={source}
                    variant={hidden ? "outlined" : "contained"}
                  >
                    {group.label}
                  </Button>
                );
              })}
            </div>
          )}
          {detachedTargetChips.length > 0 && (
            <div aria-label="Detached sources" className="detachedToggles" role="group">
              <span className="detachedLabel">Detached</span>
              {detachedTargetChips.map((chip) => (
                <Button
                  className="detachedToggle"
                  key={chip.id}
                  onClick={() => reattachTarget(chip.target)}
                  size="small"
                  title={`Reattach ${chip.label}`}
                  variant="outlined"
                >
                  {chip.label}
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
                    {mergedRecords.length} / {scopedRecords.length}
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
                        <span className="source">{recordSourceLabel(record)}</span>
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
                  const paneTarget: PopoutTarget = pane.isGroup
                    ? { key: pane.group, kind: "group" }
                    : { key: pane.childSources[0] ?? pane.group, kind: "source" };
                  const canPopOutPane = !sameTarget(paneTarget, popoutTarget);
                  return (
                    <section
                      className={`sourcePane ${pane.isGroup ? "groupPane" : ""}`}
                      key={pane.group}
                    >
                      <header
                        className={`sourcePaneHeader ${pane.isGroup ? "groupPaneHeader" : ""}`}
                      >
                        <div className="sourcePaneTitle">
                          <strong title={pane.group}>{pane.label}</strong>
                          <span>
                            {visibleCount} / {pane.total}
                          </span>
                          {pane.isGroup && (
                            <span className="sourcePaneBadge">
                              {pane.hiddenChildSources.length > 0
                                ? `${pane.visibleChildSources.length}/${pane.childSources.length} sources`
                                : `${pane.childSources.length} sources`}
                            </span>
                          )}
                        </div>
                        <div className="sourcePaneActions">
                          {pane.isGroup && (
                            <ToggleButtonGroup
                              aria-label={`${pane.label} grouping`}
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
                              aria-label={`${pane.label} child source layout`}
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
                              ariaLabel={`${pane.label} levels`}
                              onToggle={(level) => togglePaneLevel(pane.scopeKey, level)}
                              selected={pane.levels}
                            />
                          )}
                          <PaneMenu
                            autoScroll={sourceAutoScroll(autoScrollSources, pane.scopeKey)}
                            details={showLineDetails}
                            expungeLabel={pane.isGroup ? "Expunge Group" : "Expunge"}
                            label={`${pane.label} controls`}
                            onAutoScrollChange={(enabled) => toggleAutoScroll(pane.scopeKey, enabled)}
                            onBlock={
                              pane.isGroup
                                ? undefined
                                : () => blockSource(pane.childSources[0] ?? pane.group)
                            }
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
                            onPopOut={
                              canPopOutPane ? () => openPopout(paneTarget, pane.label) : undefined
                            }
                            paused={isPaused}
                            popOutLabel={pane.isGroup ? "Pop Out Group" : "Pop Out Source"}
                          />
                        </div>
                        {pane.isGroup && (
                          <div
                            aria-label={`${pane.label} child source visibility`}
                            className="childSourceToggles"
                            role="group"
                          >
                            {pane.childSourceToggles.map((child) => (
                              <span className="childSourceControl" key={child.source}>
                                <Button
                                  aria-pressed={!child.hidden}
                                  className="childSourceToggle"
                                  onClick={() =>
                                    setExcludedSources((current) =>
                                      toggleExcludedKey(current, sourceKey(child.source)),
                                    )
                                  }
                                  size="small"
                                  title={`${child.hidden ? "Show" : "Hide"} ${child.source}`}
                                  variant={child.hidden ? "outlined" : "contained"}
                                >
                                  {child.label}
                                </Button>
                                <Tooltip title={`Block ${child.source}`}>
                                  <IconButton
                                    aria-label={`Block ${child.source}`}
                                    className="sourceBlockButton"
                                    onClick={() => blockSource(child.source)}
                                    size="small"
                                  >
                                    <BlockIcon fontSize="inherit" />
                                  </IconButton>
                                </Tooltip>
                              </span>
                            ))}
                          </div>
                        )}
                      </header>
                      {pane.isGroup && pane.viewMode === "source" ? (
                        <div
                          className={`nestedSourcePaneArea ${pane.layout}`}
                          style={
                            {
                              "--nested-source-count": Math.max(1, pane.childPanes.length),
                            } as CSSProperties
                          }
                        >
                          {pane.childPanes.length === 0 ? (
                            <div className="emptyState">No visible sources.</div>
                          ) : (
                            pane.childPanes.map((child) => {
                              const childDetails = sourceLineDetails(detailSources, child.scopeKey);
                              const childPaused = sourcePaused(pausedSources, child.scopeKey);
                              const childTarget: PopoutTarget = {
                                key: child.source,
                                kind: "source",
                              };
                              const canPopOutChild = !sameTarget(childTarget, popoutTarget);
                              return (
                                <section className="nestedSourcePane" key={child.source}>
                                  <header className="nestedSourceHeader">
                                    <div className="sourcePaneTitle">
                                      <strong title={child.source}>{child.label}</strong>
                                      <span>
                                        {child.records.length} / {child.total}
                                      </span>
                                    </div>
                                    <div className="sourcePaneActions">
                                      <LevelButtons
                                        ariaLabel={`${child.label} levels`}
                                        onToggle={(level) => togglePaneLevel(child.scopeKey, level)}
                                        selected={child.levels}
                                      />
                                      <PaneMenu
                                        autoScroll={sourceAutoScroll(
                                          autoScrollSources,
                                          child.scopeKey,
                                        )}
                                        details={childDetails}
                                        expungeLabel="Expunge"
                                        label={`${child.label} controls`}
                                        onAutoScrollChange={(enabled) =>
                                          toggleAutoScroll(child.scopeKey, enabled)
                                        }
                                        onBlock={() => blockSource(child.source)}
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
                                        onPopOut={
                                          canPopOutChild
                                            ? () => openPopout(childTarget, child.label)
                                            : undefined
                                        }
                                        paused={childPaused}
                                        popOutLabel="Pop Out Source"
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
                            })
                          )}
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
                                  {showRecordSource && (
                                    <span className="source">{recordSourceLabel(record)}</span>
                                  )}
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
      <AboutDialog
        about={about}
        connection={connection}
        error={aboutError}
        onClose={() => setActiveDialog(null)}
        open={activeDialog === "about"}
      />
      <HelpDialog onClose={() => setActiveDialog(null)} open={activeDialog === "help"} />
      <BlockedSourcesDialog
        blockedSources={blockedSources}
        onClose={() => setActiveDialog(null)}
        onUnblock={unblockSource}
        open={activeDialog === "blocked"}
      />
    </ThemeProvider>
  );
}

function AboutDialog({
  about,
  connection,
  error,
  onClose,
  open,
}: {
  about: AboutResponse | null;
  connection: ConnectionState;
  error: string;
  onClose: () => void;
  open: boolean;
}) {
  return (
    <Dialog className="appDialog" fullWidth maxWidth="sm" onClose={onClose} open={open}>
      <DialogTitle>About DevLogBus</DialogTitle>
      <DialogContent>
        {error !== "" ? (
          <div className="dialogError">{error}</div>
        ) : about == null ? (
          <div className="emptyState">Loading.</div>
        ) : (
          <div className="aboutGrid">
            <KeyValue label="version" value={about.build.version} />
            <KeyValue label="commit" value={about.build.commit} />
            <KeyValue label="build date" value={about.build.buildDate} />
            <KeyValue label="go" value={about.build.goVersion ?? ""} />
            <KeyValue label="module" value={about.build.modulePath ?? ""} />
            <KeyValue label="api" value={about.api.ok ? "ok" : "offline"} />
            <KeyValue label="stream" value={connection} />
            <KeyValue label="endpoint" value={about.broker.endpoint} />
            <KeyValue label="http" value={about.broker.httpListenAddress} />
            <KeyValue label="tcp" value={about.broker.tcpListenAddress || "disabled"} />
            <KeyValue label="records/source" value={String(about.broker.maxRecords)} />
            <KeyValue label="echo" value={about.broker.echo ? "on" : "off"} />
          </div>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} variant="contained">
          Close
        </Button>
      </DialogActions>
    </Dialog>
  );
}

function HelpDialog({ onClose, open }: { onClose: () => void; open: boolean }) {
  return (
    <Dialog className="appDialog" fullWidth maxWidth="md" onClose={onClose} open={open}>
      <DialogTitle>Viewer Cheat Sheet</DialogTitle>
      <DialogContent>
        <div className="helpSheet">
          <section className="helpPanel">
            <h2>Finding Signal</h2>
            <HelpRow action="Merged" detail="One stream. Best when the whole system is still quiet." />
            <HelpRow action="By source" detail="Pane per source. Best once the noise starts." />
            <HelpRow
              action="Chrome groups"
              detail="A tab stays grouped; child hosts can be shown, hidden, blocked, or popped out."
            />
            <HelpRow
              action="Search"
              detail="Matches message, source, level, time, and structured fields."
            />
          </section>

          <section className="helpPanel">
            <h2>Noise Control</h2>
            <HelpRow action="Hide" detail="Temporary visibility toggle. Nothing is deleted." />
            <HelpRow action="Block" detail="Persistent source suppression until you unblock it." />
            <HelpRow action="Pause" detail="Stops that pane from accepting new records." />
            <HelpRow action="Levels" detail="Per-pane DEBUG, INFO, WARN, ERROR filters." />
          </section>

          <section className="helpPanel">
            <h2>Windows</h2>
            <HelpRow action="Pop out" detail="Moves a source or group into its own window." />
            <HelpRow action="Reattach" detail="Returns a detached window to the main viewer." />
            <HelpRow action="Detached chips" detail="Click a chip in the main toolbar to pull it back." />
          </section>

          <section className="helpPanel">
            <h2>Record Cleanup</h2>
            <HelpRow action="Clear" detail="Removes records from this viewer only." />
            <HelpRow action="Expunge" detail="Deletes matching replay records from the daemon buffer." />
            <HelpRow
              action="Chrome tap"
              detail="The extension posts console, runtime, log, and network events to the daemon."
            />
          </section>
        </div>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} variant="contained">
          Close
        </Button>
      </DialogActions>
    </Dialog>
  );
}

function BlockedSourcesDialog({
  blockedSources,
  onClose,
  onUnblock,
  open,
}: {
  blockedSources: string[];
  onClose: () => void;
  onUnblock: (source: string) => void;
  open: boolean;
}) {
  return (
    <Dialog className="appDialog" fullWidth maxWidth="sm" onClose={onClose} open={open}>
      <DialogTitle>Blocked Sources</DialogTitle>
      <DialogContent>
        {blockedSources.length === 0 ? (
          <div className="emptyState">No blocked sources.</div>
        ) : (
          <div className="blockedSourceList">
            {blockedSources.map((source) => (
              <div className="blockedSourceRow" key={source}>
                <span title={source}>{source}</span>
                <Button onClick={() => onUnblock(source)} size="small" variant="outlined">
                  Unblock
                </Button>
              </div>
            ))}
          </div>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} variant="contained">
          Close
        </Button>
      </DialogActions>
    </Dialog>
  );
}

function HelpRow({ action, detail }: { action: string; detail: string }) {
  return (
    <div className="helpRow">
      <strong>{action}</strong>
      <span>{detail}</span>
    </div>
  );
}

function KeyValue({ label, value }: { label: string; value: string }) {
  return (
    <div className="keyValue">
      <span>{label}</span>
      <strong>{value.trim() === "" ? "unknown" : value}</strong>
    </div>
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
  onBlock,
  onClear,
  onDetailsChange,
  onExpunge,
  onPauseChange,
  onPopOut,
  paused,
  popOutLabel = "Pop Out",
}: {
  autoScroll: boolean;
  details: boolean;
  expungeLabel: string;
  label: string;
  onAutoScrollChange: (enabled: boolean) => void;
  onBlock?: () => void;
  onClear: () => void;
  onDetailsChange: (enabled: boolean) => void;
  onExpunge: () => void;
  onPauseChange: (enabled: boolean) => void;
  onPopOut?: () => void;
  paused: boolean;
  popOutLabel?: string;
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
        {onPopOut != null && (
          <MenuItem
            className="paneMenuItem"
            onClick={() => {
              onPopOut();
              closeMenu();
            }}
          >
            <ListItemIcon>
              <OpenInNewIcon fontSize="small" />
            </ListItemIcon>
            <ListItemText>{popOutLabel}</ListItemText>
          </MenuItem>
        )}
        {onBlock != null && (
          <MenuItem
            className="paneMenuItem destructive"
            onClick={() => {
              onBlock();
              closeMenu();
            }}
          >
            <ListItemIcon>
              <BlockIcon color="error" fontSize="small" />
            </ListItemIcon>
            <ListItemText>Block Source</ListItemText>
          </MenuItem>
        )}
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
