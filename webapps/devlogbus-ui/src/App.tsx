type LogLevel = "DEBUG" | "INFO" | "WARN" | "ERROR";

type LogRecord = {
  id: string;
  time: string;
  level: LogLevel;
  source: string;
  message: string;
  attrs: Record<string, string>;
};

const records: LogRecord[] = [
  {
    id: "1",
    time: "14:22:01.318",
    level: "INFO",
    source: "event_management_svc",
    message: "Connecting to database",
    attrs: { host: "invictus", db: "event_core", user: "event_management_service" },
  },
  {
    id: "2",
    time: "14:22:01.955",
    level: "WARN",
    source: "billing_svc",
    message: "event catalog unavailable",
    attrs: { service: "billing_svc", error: "dial tcp 127.0.0.1:50052: connection refused" },
  },
  {
    id: "3",
    time: "14:22:02.027",
    level: "INFO",
    source: "tenant_data_svc",
    message: "RPC server listening",
    attrs: { socket: "/tmp/devlogbus/devlogbus.sock" },
  },
];

const levelClass: Record<LogLevel, string> = {
  DEBUG: "debug",
  INFO: "info",
  WARN: "warn",
  ERROR: "error",
};

export default function App() {
  const selected = records[1];

  return (
    <main className="shell">
      <header className="topbar">
        <div>
          <h1>DevLogBus</h1>
          <p>Live structured logs for local development.</p>
        </div>
        <div className="status">
          <span className="dot" />
          broker online
        </div>
      </header>

      <section className="toolbar" aria-label="Log filters">
        <input aria-label="Search logs" placeholder="Search message, source, or field" />
        <select aria-label="Minimum level" defaultValue="DEBUG">
          <option>DEBUG</option>
          <option>INFO</option>
          <option>WARN</option>
          <option>ERROR</option>
        </select>
        <select aria-label="Source" defaultValue="all">
          <option value="all">all sources</option>
          <option>event_management_svc</option>
          <option>billing_svc</option>
          <option>tenant_data_svc</option>
        </select>
        <button type="button">Pause</button>
      </section>

      <section className="content">
        <div className="logList" aria-label="Live log records">
          {records.map((record) => (
            <button className={`logRow ${record.id === selected.id ? "selected" : ""}`} key={record.id} type="button">
              <span className="time">{record.time}</span>
              <span className={`level ${levelClass[record.level]}`}>{record.level}</span>
              <span className="source">{record.source}</span>
              <span className="message">{record.message}</span>
            </button>
          ))}
        </div>

        <aside className="detail" aria-label="Selected record fields">
          <div className="detailHeader">
            <span className={`level ${levelClass[selected.level]}`}>{selected.level}</span>
            <strong>{selected.message}</strong>
          </div>
          <dl>
            <div>
              <dt>time</dt>
              <dd>{selected.time}</dd>
            </div>
            <div>
              <dt>source</dt>
              <dd>{selected.source}</dd>
            </div>
            {Object.entries(selected.attrs).map(([key, value]) => (
              <div key={key}>
                <dt>{key}</dt>
                <dd>{value}</dd>
              </div>
            ))}
          </dl>
        </aside>
      </section>
    </main>
  );
}
