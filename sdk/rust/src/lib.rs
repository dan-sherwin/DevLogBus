use std::fmt;
use std::io::{Read, Write};
use std::net::TcpStream;
use std::sync::Arc;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

pub const DEFAULT_HTTP_ENDPOINT: &str = "http://127.0.0.1:7423";
pub const REDACTED_VALUE: &str = "[REDACTED]";

pub type RecordFilter = Arc<dyn Fn(&Record) -> bool + Send + Sync>;
pub type RecordRedactor = Arc<dyn Fn(&mut Record) + Send + Sync>;

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Record {
    pub time: Option<String>,
    pub level: String,
    pub source: String,
    pub message: String,
    pub attrs_json: Option<String>,
}

impl Record {
    pub fn new(
        source: impl Into<String>,
        level: impl Into<String>,
        message: impl Into<String>,
    ) -> Self {
        Self {
            time: None,
            level: level.into(),
            source: source.into(),
            message: message.into(),
            attrs_json: None,
        }
    }

    pub fn attrs_json(mut self, attrs_json: impl Into<String>) -> Self {
        self.attrs_json = Some(attrs_json.into());
        self
    }
}

#[derive(Clone)]
pub struct Client {
    endpoint: String,
    source: String,
    timeout: Duration,
    filter: Option<RecordFilter>,
    redactor: Option<RecordRedactor>,
}

#[derive(Default)]
pub struct ClientOptions {
    pub endpoint: Option<String>,
    pub source: Option<String>,
    pub timeout: Option<Duration>,
    pub filter: Option<RecordFilter>,
    pub redactor: Option<RecordRedactor>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct PublishResult {
    pub published: u32,
    pub filtered: bool,
}

#[derive(Debug)]
pub enum Error {
    InvalidEndpoint(String),
    InvalidRecord(String),
    Io(std::io::Error),
    Http(u16),
}

impl fmt::Display for Error {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Error::InvalidEndpoint(message) => write!(f, "{message}"),
            Error::InvalidRecord(message) => write!(f, "{message}"),
            Error::Io(err) => write!(f, "{err}"),
            Error::Http(status) => write!(f, "DevLogBus publish failed: HTTP {status}"),
        }
    }
}

impl std::error::Error for Error {}

impl From<std::io::Error> for Error {
    fn from(value: std::io::Error) -> Self {
        Error::Io(value)
    }
}

impl Client {
    pub fn new(options: ClientOptions) -> Self {
        Self {
            endpoint: trim_endpoint(options.endpoint.as_deref().unwrap_or(DEFAULT_HTTP_ENDPOINT)),
            source: options.source.unwrap_or_default(),
            timeout: options.timeout.unwrap_or(Duration::from_secs(2)),
            filter: options.filter,
            redactor: options.redactor,
        }
    }

    pub fn publish_message(
        &self,
        level: impl Into<String>,
        message: impl Into<String>,
        attrs_json: Option<String>,
    ) -> Result<PublishResult, Error> {
        let mut record = Record::new(self.source.clone(), level, message);
        record.attrs_json = attrs_json;
        self.publish(record)
    }

    pub fn publish(&self, mut record: Record) -> Result<PublishResult, Error> {
        if record.source.trim().is_empty() {
            record.source = self.source.clone();
        }
        if let Some(filter) = &self.filter {
            if !filter(&record) {
                return Ok(PublishResult {
                    published: 0,
                    filtered: true,
                });
            }
        }
        if let Some(redactor) = &self.redactor {
            redactor(&mut record);
        }

        let payload = record_json(&record)?;
        let endpoint = parse_endpoint(&self.endpoint)?;
        let mut stream = TcpStream::connect((endpoint.host.as_str(), endpoint.port))?;
        stream.set_read_timeout(Some(self.timeout))?;
        stream.set_write_timeout(Some(self.timeout))?;

        let request = format!(
            "POST {} HTTP/1.1\r\nHost: {}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
            endpoint.path,
            endpoint.host_header(),
            payload.len(),
            payload
        );
        stream.write_all(request.as_bytes())?;
        stream.flush()?;

        let mut response = String::new();
        stream.read_to_string(&mut response)?;
        let status = parse_status(&response)?;
        if !(200..300).contains(&status) {
            return Err(Error::Http(status));
        }
        Ok(PublishResult {
            published: 1,
            filtered: false,
        })
    }
}

pub fn drop_sources<I, S>(sources: I) -> RecordFilter
where
    I: IntoIterator<Item = S>,
    S: Into<String>,
{
    let blocked: Vec<String> = sources
        .into_iter()
        .map(Into::into)
        .map(|s| s.trim().to_string())
        .filter(|s| !s.is_empty())
        .collect();
    Arc::new(move |record: &Record| !blocked.iter().any(|source| source == &record.source))
}

pub fn redact_message() -> RecordRedactor {
    Arc::new(|record: &mut Record| {
        record.message = REDACTED_VALUE.to_string();
    })
}

pub fn normalize_level(level: &str) -> String {
    match level.trim().to_ascii_lowercase().as_str() {
        "debug" | "dbg" => "DEBUG".to_string(),
        "" | "info" => "INFO".to_string(),
        "warn" | "warning" => "WARN".to_string(),
        "error" | "err" => "ERROR".to_string(),
        _ => level.trim().to_ascii_uppercase(),
    }
}

pub fn record_json(record: &Record) -> Result<String, Error> {
    let source = record.source.trim();
    if source.is_empty() {
        return Err(Error::InvalidRecord(
            "DevLogBus source is required".to_string(),
        ));
    }
    if record.message.is_empty() {
        return Err(Error::InvalidRecord(
            "DevLogBus message is required".to_string(),
        ));
    }
    if let Some(attrs) = &record.attrs_json {
        if !attrs.trim_start().starts_with('{') {
            return Err(Error::InvalidRecord(
                "DevLogBus attrs_json must be a JSON object".to_string(),
            ));
        }
    }

    let time = record.time.clone().unwrap_or_else(now_iso);
    let mut out = String::new();
    out.push('{');
    out.push_str("\"time\":");
    push_json_string(&mut out, &time);
    out.push_str(",\"level\":");
    push_json_string(&mut out, &normalize_level(&record.level));
    out.push_str(",\"source\":");
    push_json_string(&mut out, source);
    out.push_str(",\"message\":");
    push_json_string(&mut out, &record.message);
    if let Some(attrs) = &record.attrs_json {
        out.push_str(",\"attrs\":");
        out.push_str(attrs);
    }
    out.push('}');
    Ok(out)
}

struct Endpoint {
    host: String,
    port: u16,
    path: String,
}

impl Endpoint {
    fn host_header(&self) -> String {
        format!("{}:{}", self.host, self.port)
    }
}

fn parse_endpoint(endpoint: &str) -> Result<Endpoint, Error> {
    let raw = endpoint.trim().trim_end_matches('/');
    let raw = raw.strip_prefix("http://").ok_or_else(|| {
        Error::InvalidEndpoint("DevLogBus Rust SDK only supports http:// endpoints".to_string())
    })?;
    let (authority, base_path) = match raw.split_once('/') {
        Some((authority, path)) => (authority, format!("/{path}")),
        None => (raw, String::new()),
    };
    let (host, port) = match authority.rsplit_once(':') {
        Some((host, port)) => {
            let parsed = port.parse::<u16>().map_err(|_| {
                Error::InvalidEndpoint("DevLogBus endpoint port is invalid".to_string())
            })?;
            (host.to_string(), parsed)
        }
        None => (authority.to_string(), 80),
    };
    if host.is_empty() {
        return Err(Error::InvalidEndpoint(
            "DevLogBus endpoint host is required".to_string(),
        ));
    }
    Ok(Endpoint {
        host,
        port,
        path: format!("{}/api/records", base_path.trim_end_matches('/')),
    })
}

fn parse_status(response: &str) -> Result<u16, Error> {
    let status = response
        .lines()
        .next()
        .and_then(|line| line.split_whitespace().nth(1))
        .and_then(|value| value.parse::<u16>().ok())
        .ok_or_else(|| {
            Error::InvalidEndpoint("DevLogBus response did not include an HTTP status".to_string())
        })?;
    Ok(status)
}

fn trim_endpoint(endpoint: &str) -> String {
    endpoint.trim().trim_end_matches('/').to_string()
}

fn push_json_string(out: &mut String, value: &str) {
    out.push('"');
    for ch in value.chars() {
        match ch {
            '"' => out.push_str("\\\""),
            '\\' => out.push_str("\\\\"),
            '\n' => out.push_str("\\n"),
            '\r' => out.push_str("\\r"),
            '\t' => out.push_str("\\t"),
            '\u{08}' => out.push_str("\\b"),
            '\u{0c}' => out.push_str("\\f"),
            ch if ch < '\u{20}' => out.push_str(&format!("\\u{:04x}", ch as u32)),
            ch => out.push(ch),
        }
    }
    out.push('"');
}

fn now_iso() -> String {
    let secs = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_secs())
        .unwrap_or(0);
    format_unix_utc(secs)
}

fn format_unix_utc(secs: u64) -> String {
    let days = (secs / 86_400) as i64;
    let seconds_of_day = secs % 86_400;
    let hour = seconds_of_day / 3_600;
    let minute = (seconds_of_day % 3_600) / 60;
    let second = seconds_of_day % 60;
    let (year, month, day) = civil_from_days(days);
    format!("{year:04}-{month:02}-{day:02}T{hour:02}:{minute:02}:{second:02}Z")
}

fn civil_from_days(days_since_epoch: i64) -> (i64, u64, u64) {
    let z = days_since_epoch + 719_468;
    let era = if z >= 0 { z } else { z - 146_096 } / 146_097;
    let doe = z - era * 146_097;
    let yoe = (doe - doe / 1_460 + doe / 36_524 - doe / 146_096) / 365;
    let y = yoe + era * 400;
    let doy = doe - (365 * yoe + yoe / 4 - yoe / 100);
    let mp = (5 * doy + 2) / 153;
    let day = doy - (153 * mp + 2) / 5 + 1;
    let month = mp + if mp < 10 { 3 } else { -9 };
    let year = y + if month <= 2 { 1 } else { 0 };
    (year, month as u64, day as u64)
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::{Read, Write};
    use std::net::TcpListener;
    use std::thread;

    #[test]
    fn normalizes_levels() {
        assert_eq!(normalize_level("warning"), "WARN");
        assert_eq!(normalize_level("dbg"), "DEBUG");
        assert_eq!(normalize_level("custom"), "CUSTOM");
    }

    #[test]
    fn formats_unix_time_as_rfc3339_utc() {
        assert_eq!(format_unix_utc(0), "1970-01-01T00:00:00Z");
        assert_eq!(format_unix_utc(1_704_067_199), "2023-12-31T23:59:59Z");
    }

    #[test]
    fn builds_record_json() {
        let record = Record {
            time: Some("2026-05-31T12:00:00Z".to_string()),
            level: "warning".to_string(),
            source: "rust_test".to_string(),
            message: "quote \" newline\n".to_string(),
            attrs_json: Some("{\"request\":{\"id\":\"req-1\"}}".to_string()),
        };

        let json = record_json(&record).unwrap();

        assert!(json.contains("\"level\":\"WARN\""));
        assert!(json.contains("\"source\":\"rust_test\""));
        assert!(json.contains("quote \\\" newline\\n"));
        assert!(json.contains("\"attrs\":{\"request\":{\"id\":\"req-1\"}}"));
    }

    #[test]
    fn rejects_non_object_attrs_json() {
        let record = Record::new("rust_test", "INFO", "hello").attrs_json("\"nope\"");
        assert!(record_json(&record).is_err());
    }

    #[test]
    fn filters_before_http_publish() {
        let client = Client::new(ClientOptions {
            endpoint: Some("http://127.0.0.1:1".to_string()),
            source: Some("hidden".to_string()),
            filter: Some(drop_sources(["hidden"])),
            ..Default::default()
        });

        let result = client.publish_message("INFO", "drop me", None).unwrap();

        assert_eq!(
            result,
            PublishResult {
                published: 0,
                filtered: true
            }
        );
    }

    #[test]
    fn publishes_to_http_endpoint() {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let port = listener.local_addr().unwrap().port();
        let handle = thread::spawn(move || {
            let (mut stream, _) = listener.accept().unwrap();
            let mut buf = [0_u8; 1024];
            let mut bytes = Vec::new();
            loop {
                let n = stream.read(&mut buf).unwrap();
                bytes.extend_from_slice(&buf[..n]);
                if bytes.windows(4).any(|window| window == b"\r\n\r\n") {
                    break;
                }
            }
            let header_end = bytes
                .windows(4)
                .position(|window| window == b"\r\n\r\n")
                .unwrap()
                + 4;
            let headers = String::from_utf8_lossy(&bytes[..header_end]).to_string();
            let content_length = headers
                .lines()
                .find_map(|line| line.strip_prefix("Content-Length: "))
                .and_then(|value| value.parse::<usize>().ok())
                .unwrap_or(0);
            while bytes.len() < header_end + content_length {
                let n = stream.read(&mut buf).unwrap();
                bytes.extend_from_slice(&buf[..n]);
            }
            let request = String::from_utf8_lossy(&bytes).to_string();
            stream.write_all(b"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: 15\r\nConnection: close\r\n\r\n{\"published\":1}").unwrap();
            request
        });

        let client = Client::new(ClientOptions {
            endpoint: Some(format!("http://127.0.0.1:{port}")),
            source: Some("rust_test".to_string()),
            redactor: Some(redact_message()),
            ..Default::default()
        });

        let result = client.publish_message("INFO", "secret", None).unwrap();
        let request = handle.join().unwrap();

        assert_eq!(
            result,
            PublishResult {
                published: 1,
                filtered: false
            }
        );
        assert!(request.contains("POST /api/records HTTP/1.1"));
        assert!(request.contains(REDACTED_VALUE));
    }
}
