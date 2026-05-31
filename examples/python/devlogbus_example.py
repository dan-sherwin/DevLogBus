#!/usr/bin/env python3
import json
import urllib.request
from datetime import datetime, timezone


ENDPOINT = "http://127.0.0.1:7423"


def publish(record):
    data = json.dumps(record).encode("utf-8")
    request = urllib.request.Request(
        f"{ENDPOINT}/api/records",
        data=data,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with urllib.request.urlopen(request, timeout=2) as response:
        if response.status != 200:
            raise RuntimeError(f"DevLogBus publish failed: {response.status}")


def now():
    return datetime.now(timezone.utc).isoformat()


publish(
    {
        "time": now(),
        "level": "INFO",
        "source": "example_python",
        "message": "worker started",
        "attrs": {"queue": "demo"},
    }
)

publish(
    {
        "time": now(),
        "level": "ERROR",
        "source": "example_python",
        "message": "job failed",
        "attrs": {"job_id": "demo-job", "attempt": 2},
    }
)
