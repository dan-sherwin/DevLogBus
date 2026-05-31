# Python SDK

The Python SDK lives in:

```text
sdk/python
```

It publishes to the DevLogBus HTTP API using only the Python standard library.
The default endpoint is:

```text
http://127.0.0.1:7423
```

Install from a checkout or release archive:

```bash
python3 -m pip install /path/to/DevLogBus/sdk/python
```

The package name is `devlogbus` for PyPI publishing.

## Client

```python
from devlogbus import DevLogBusClient

devlog = DevLogBusClient(source="checkout_worker")

devlog.publish(
    level="INFO",
    message="checkout started",
    attrs={"request_id": "req-1"},
)
```

Pass `endpoint` explicitly for a different local or trusted-network daemon:

```python
devlog = DevLogBusClient(
    endpoint="http://devbox:7423",
    source="checkout_worker",
)
```

## Logging Handler

```python
import logging
from devlogbus import DevLogBusLoggingHandler

logger = logging.getLogger("checkout")
logger.addHandler(DevLogBusLoggingHandler(source="checkout_worker"))
logger.warning("payment provider slow")
```

## Filters And Redaction

Filters drop records before publishing. Redactors return the record shape that
will be sent to the daemon.

```python
from devlogbus import DevLogBusClient, drop_sources, redact_attrs

devlog = DevLogBusClient(
    source="checkout_worker",
    filter=drop_sources(["noisy_worker"]),
    redactor=redact_attrs(["authorization", "token", "request.apiKey"]),
)
```

`redact_attrs` matches either an attribute key or dotted nested path and
replaces matching values with `[REDACTED]`.

## Local Test

From the repository root:

```bash
python3 -m unittest discover -s sdk/python/tests
```
