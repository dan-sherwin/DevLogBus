from __future__ import annotations

import json
import logging
import sys
import threading
import unittest
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path


sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from devlogbus import (  # noqa: E402
    REDACTED_VALUE,
    DevLogBusClient,
    DevLogBusLoggingHandler,
    drop_sources,
    normalize_level,
    redact_attrs,
)


class CaptureHandler(BaseHTTPRequestHandler):
    records: list[dict] = []

    def do_POST(self) -> None:
        if self.path != "/api/records":
            self.send_response(404)
            self.end_headers()
            return
        length = int(self.headers["Content-Length"])
        CaptureHandler.records.append(json.loads(self.rfile.read(length).decode("utf-8")))
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(b'{"published":1}')

    def log_message(self, format: str, *args: object) -> None:
        return


class DevLogBusClientTest(unittest.TestCase):
    def setUp(self) -> None:
        CaptureHandler.records = []
        self.server = HTTPServer(("127.0.0.1", 0), CaptureHandler)
        self.thread = threading.Thread(target=self.server.serve_forever)
        self.thread.start()
        self.endpoint = f"http://127.0.0.1:{self.server.server_address[1]}"

    def tearDown(self) -> None:
        self.server.shutdown()
        self.server.server_close()
        self.thread.join()

    def test_normalizes_levels(self) -> None:
        self.assertEqual(normalize_level("warning"), "WARN")
        self.assertEqual(normalize_level("dbg"), "DEBUG")
        self.assertEqual(normalize_level("custom"), "CUSTOM")

    def test_publishes_records(self) -> None:
        client = DevLogBusClient(endpoint=self.endpoint, source="python-test")

        result = client.publish(message="hello", level="warn", attrs={"request_id": "req-1"})

        self.assertEqual(result["published"], 1)
        self.assertEqual(len(CaptureHandler.records), 1)
        self.assertEqual(CaptureHandler.records[0]["source"], "python-test")
        self.assertEqual(CaptureHandler.records[0]["level"], "WARN")
        self.assertEqual(CaptureHandler.records[0]["attrs"]["request_id"], "req-1")

    def test_filters_before_publishing(self) -> None:
        client = DevLogBusClient(
            endpoint=self.endpoint,
            source="hidden",
            filter=drop_sources(["hidden"]),
        )

        result = client.publish(message="drop me")

        self.assertEqual(result, {"published": 0, "filtered": True})
        self.assertEqual(CaptureHandler.records, [])

    def test_redacts_attrs_before_publishing(self) -> None:
        client = DevLogBusClient(
            endpoint=self.endpoint,
            source="python-test",
            redactor=redact_attrs(["token", "request.authorization"]),
        )

        client.publish(
            message="hello",
            attrs={
                "token": "secret",
                "request": {
                    "authorization": "Bearer secret",
                    "id": "req-1",
                },
            },
        )

        attrs = CaptureHandler.records[0]["attrs"]
        self.assertEqual(attrs["token"], REDACTED_VALUE)
        self.assertEqual(attrs["request"]["authorization"], REDACTED_VALUE)
        self.assertEqual(attrs["request"]["id"], "req-1")

    def test_logging_handler_publishes_log_records(self) -> None:
        logger = logging.getLogger("devlogbus-python-test")
        logger.handlers = []
        logger.propagate = False
        logger.setLevel(logging.INFO)
        logger.addHandler(DevLogBusLoggingHandler(endpoint=self.endpoint, source="python-logger"))

        logger.warning("slow request")

        self.assertEqual(CaptureHandler.records[0]["source"], "python-logger")
        self.assertEqual(CaptureHandler.records[0]["level"], "WARN")
        self.assertEqual(CaptureHandler.records[0]["message"], "slow request")
        self.assertEqual(CaptureHandler.records[0]["attrs"]["logger"], "devlogbus-python-test")


if __name__ == "__main__":
    unittest.main()
