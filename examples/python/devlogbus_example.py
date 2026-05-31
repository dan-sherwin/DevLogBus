#!/usr/bin/env python3
from pathlib import Path
import sys


sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "sdk" / "python"))

from devlogbus import DevLogBusClient, redact_attrs  # noqa: E402


devlog = DevLogBusClient(
    source="example_python",
    redactor=redact_attrs(["authorization", "token"]),
)

devlog.publish(
    level="INFO",
    message="worker started",
    attrs={"queue": "demo"},
)

devlog.publish(
    level="ERROR",
    message="job failed",
    attrs={"job_id": "demo-job", "attempt": 2},
)
