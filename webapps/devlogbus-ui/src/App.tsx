import { useEffect, useMemo, useRef, useState } from "react";

type LogLevel = "DEBUG" | "INFO" | "WARN" | "ERROR";
type ConnectionState = "connecting" | "online" | "reconnecting";

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
  (import.meta as ViteImportMeta).env?.VITE_DEVLOGBUS_API_URL ?? "http://127.0.0.1:7423"
).replace(/\/$/, "");
const maxVisibleRecords = 1000;
const levels: LogLevel[] = ["DEBUG", "INFO", "WARN", "ERROR"];
const levelValue: Record<LogLevel, number> = {
  DEBUG: -4,
  INFO: 0,
  WARN: 4,
  ERROR: 8,
};
const levelClass: Record<LogLevel, string> = {
  DEBUG: "debug",
  INFO: "info",
  WARN: "warn",
  ERROR: "error",
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
  return Array.from(next.values()).slice(-maxVisibleRecords);
}

export default function App() {
  const [records, setRecords] = useState<LogRecord[]>([]);
  const [connection, setConnection] = useState<ConnectionState>("connecting");
  const [paused, setPaused] = useState(false);
  const [search, setSearch] = useState("");
  const [minimumLevel, setMinimumLevel] = useState<LogLevel>("DEBUG");
  const [sourceFilter, setSourceFilter] = useState("all");
  const [selectedID, setSelectedID] = useState("");
  const pausedRef = useRef(paused);

  useEffect(() => {
    pausedRef.current = paused;
  }, [paused]);

  useEffect(() => {
    const params = new URLSearchParams({ replay: "500", level: "debug" });
    const stream = new EventSource(`${apiBase}/api/stream?${params.toString()}`);

    stream.onopen = () => setConnection("online");
    stream.onerror = () => setConnection("reconnecting");
    stream.addEventListener("record", (event) => {
      if (pausedRef.current) {
        return;
      }
      try {
        const record = JSON.parse((event as MessageEvent<string>).data) as LogRecord;
        setRecords((current) => mergeRecord(current, record));
      } catch (error) {
        console.error("Failed to parse DevLogBus record", error);
      }
    });

    return () => stream.close();
  }, []);

  const sources = useMemo(
    () => Array.from(new Set(records.map((record) => record.source).filter(Boolean))).sort(),
    [records],
  );

  const filteredRecords = useMemo(() => {
    const query = search.trim().toLowerCase();
    return records.filter((record) => {
      const normalized = normalizeLevel(record.level);
      if (levelValue[normalized] < levelValue[minimumLevel]) {
        return false;
      }
      if (sourceFilter !== "all" && record.source !== sourceFilter) {
        return false;
      }
      if (query !== "" && !searchableText(record).includes(query)) {
        return false;
      }
      return true;
    });
  }, [minimumLevel, records, search, sourceFilter]);

  const selected =
    filteredRecords.find((record) => record.id === selectedID) ?? filteredRecords.at(-1) ?? null;

  const clearRecords = () => {
    setRecords([]);
    setSelectedID("");
  };

  return (
    <main className="shell">
      <header className="topbar">
        <div>
          <h1>DevLogBus</h1>
          <p>
            {filteredRecords.length} shown / {records.length} buffered
          </p>
        </div>
        <div className={`status ${connection}`}>
          <span className="dot" />
          broker {connection}
        </div>
      </header>

      <section className="toolbar" aria-label="Log filters">
        <input
          aria-label="Search logs"
          onChange={(event) => setSearch(event.target.value)}
          placeholder="Search message, source, or field"
          value={search}
        />
        <select
          aria-label="Minimum level"
          onChange={(event) => setMinimumLevel(event.target.value as LogLevel)}
          value={minimumLevel}
        >
          {levels.map((level) => (
            <option key={level}>{level}</option>
          ))}
        </select>
        <select
          aria-label="Source"
          onChange={(event) => setSourceFilter(event.target.value)}
          value={sourceFilter}
        >
          <option value="all">all sources</option>
          {sources.map((source) => (
            <option key={source}>{source}</option>
          ))}
        </select>
        <button onClick={() => setPaused((value) => !value)} type="button">
          {paused ? "Resume" : "Pause"}
        </button>
        <button onClick={clearRecords} type="button">
          Clear
        </button>
      </section>

      <section className="content">
        <div className="logList" aria-label="Live log records">
          {filteredRecords.length === 0 ? (
            <div className="emptyState">Waiting for records.</div>
          ) : (
            filteredRecords.map((record) => {
              const level = normalizeLevel(record.level);
              const isSelected = selected?.id === record.id;
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
                  <span className="message">{record.message}</span>
                </button>
              );
            })
          )}
        </div>

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
      </section>
    </main>
  );
}
