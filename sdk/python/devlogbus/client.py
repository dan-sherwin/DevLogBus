from __future__ import annotations

import json
import logging
import urllib.error
import urllib.request
from collections.abc import Callable, Mapping
from datetime import datetime, timezone
from typing import Any, Optional, Sequence, Union


DEFAULT_HTTP_ENDPOINT = "http://127.0.0.1:7423"
REDACTED_VALUE = "[REDACTED]"

Record = dict[str, Any]
RecordFilter = Callable[[Record], bool]
RecordRedactor = Callable[[Record], Record]


def normalize_level(level: Optional[str] = "INFO") -> str:
    value = (level or "").strip()
    lower = value.lower()
    if lower in {"debug", "dbg"}:
        return "DEBUG"
    if lower in {"info", ""}:
        return "INFO"
    if lower in {"warn", "warning"}:
        return "WARN"
    if lower in {"error", "err"}:
        return "ERROR"
    return value.upper()


def create_record(
    *,
    source: str,
    message: str,
    level: Optional[str] = "INFO",
    attrs: Optional[Mapping[str, Any]] = None,
    time: Optional[Union[datetime, str]] = None,
) -> Record:
    source = source.strip()
    if source == "":
        raise ValueError("DevLogBus source is required")
    if message == "":
        raise ValueError("DevLogBus message is required")

    record: Record = {
        "time": format_time(time),
        "level": normalize_level(level),
        "source": source,
        "message": message,
    }
    if attrs:
        record["attrs"] = dict(attrs)
    return record


class DevLogBusClient:
    def __init__(
        self,
        *,
        endpoint: str = DEFAULT_HTTP_ENDPOINT,
        source: str = "",
        timeout: float = 2.0,
        filter: Optional[RecordFilter] = None,
        redactor: Optional[RecordRedactor] = None,
    ) -> None:
        self.endpoint = endpoint.rstrip("/")
        self.source = source
        self.timeout = timeout
        self.filter = filter
        self.redactor = redactor

    def publish(
        self,
        record: Optional[Mapping[str, Any]] = None,
        *,
        source: Optional[str] = None,
        message: Optional[str] = None,
        level: Optional[str] = "INFO",
        attrs: Optional[Mapping[str, Any]] = None,
        time: Optional[Union[datetime, str]] = None,
    ) -> dict[str, Any]:
        prepared = self._prepare(record, source=source, message=message, level=level, attrs=attrs, time=time)
        if prepared is None:
            return {"published": 0, "filtered": True}
        return self._post(prepared)

    def publish_batch(self, records: list[Mapping[str, Any]]) -> dict[str, Any]:
        prepared = [self._prepare(record) for record in records]
        publishable = [record for record in prepared if record is not None]
        if not publishable:
            return {"published": 0, "filtered": True}
        return self._post({"records": publishable})

    def logging_handler(self, *, source: Optional[str] = None, level: int = logging.NOTSET) -> "DevLogBusLoggingHandler":
        return DevLogBusLoggingHandler(client=self, source=source or self.source, level=level)

    def _prepare(
        self,
        record: Optional[Mapping[str, Any]] = None,
        *,
        source: Optional[str] = None,
        message: Optional[str] = None,
        level: Optional[str] = "INFO",
        attrs: Optional[Mapping[str, Any]] = None,
        time: Optional[Union[datetime, str]] = None,
    ) -> Optional[Record]:
        if record is None:
            out = create_record(
                source=source or self.source,
                message=message or "",
                level=level,
                attrs=attrs,
                time=time,
            )
        else:
            out = create_record(
                source=str(record.get("source") or source or self.source),
                message=str(record.get("message") or message or ""),
                level=str(record.get("level") or level or "INFO"),
                attrs=record.get("attrs") or attrs,
                time=record.get("time") or time,
            )

        if self.filter is not None and not self.filter(out):
            return None
        if self.redactor is not None:
            out = self.redactor(out)
        return validate_record(out)

    def _post(self, payload: Mapping[str, Any]) -> dict[str, Any]:
        data = json.dumps(payload, separators=(",", ":")).encode("utf-8")
        request = urllib.request.Request(
            f"{self.endpoint}/api/records",
            data=data,
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        try:
            with urllib.request.urlopen(request, timeout=self.timeout) as response:
                body = response.read().decode("utf-8")
                return json.loads(body) if body else {}
        except urllib.error.HTTPError as exc:
            raise RuntimeError(f"DevLogBus publish failed: {exc.code}") from exc


class DevLogBusLoggingHandler(logging.Handler):
    def __init__(
        self,
        *,
        client: Optional[DevLogBusClient] = None,
        endpoint: str = DEFAULT_HTTP_ENDPOINT,
        source: str = "",
        timeout: float = 2.0,
        filter: Optional[RecordFilter] = None,
        redactor: Optional[RecordRedactor] = None,
        level: int = logging.NOTSET,
    ) -> None:
        super().__init__(level)
        self.client = client or DevLogBusClient(
            endpoint=endpoint,
            source=source,
            timeout=timeout,
            filter=filter,
            redactor=redactor,
        )
        self.source = source or self.client.source

    def emit(self, record: logging.LogRecord) -> None:
        try:
            self.client.publish(
                source=self.source or record.name,
                level=record.levelname,
                message=record.getMessage(),
                attrs={
                    "logger": record.name,
                    "module": record.module,
                    "function": record.funcName,
                    "line": record.lineno,
                },
                time=datetime.fromtimestamp(record.created, timezone.utc),
            )
        except Exception:
            self.handleError(record)


def drop_sources(sources: Sequence[str]) -> RecordFilter:
    blocked = {source.strip() for source in sources if source.strip()}
    return lambda record: record.get("source") not in blocked


def redact_attrs(keys: Sequence[str], replacement: Any = REDACTED_VALUE) -> RecordRedactor:
    matchers = {key.strip().lower() for key in keys if key.strip()}

    def redactor(record: Record) -> Record:
        attrs = record.get("attrs")
        if not isinstance(attrs, dict) or not matchers:
            return record
        out = dict(record)
        out["attrs"] = redact_map(attrs, "", matchers, replacement)
        return out

    return redactor


def format_time(value: Optional[Union[datetime, str]]) -> str:
    if isinstance(value, datetime):
        if value.tzinfo is None:
            value = value.replace(tzinfo=timezone.utc)
        return value.astimezone(timezone.utc).isoformat().replace("+00:00", "Z")
    if isinstance(value, str) and value.strip() != "":
        return value
    return datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")


def validate_record(record: Mapping[str, Any]) -> Record:
    source = str(record.get("source") or "").strip()
    message = str(record.get("message") or "")
    if source == "":
        raise ValueError("DevLogBus source is required")
    if message == "":
        raise ValueError("DevLogBus message is required")
    out = dict(record)
    out["source"] = source
    out["message"] = message
    out["level"] = normalize_level(str(record.get("level") or "INFO"))
    out["time"] = format_time(record.get("time"))
    return out


def redact_map(attrs: Mapping[str, Any], prefix: str, matchers: set[str], replacement: Any) -> dict[str, Any]:
    out: dict[str, Any] = {}
    for key, value in attrs.items():
        path = f"{prefix}.{key}" if prefix else key
        if key.lower() in matchers or path.lower() in matchers:
            out[key] = replacement
        elif isinstance(value, dict):
            out[key] = redact_map(value, path, matchers, replacement)
        else:
            out[key] = value
    return out
