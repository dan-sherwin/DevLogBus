# DevLogBus Python SDK

Standard-library Python SDK for publishing records to the DevLogBus HTTP API.

```bash
python3 -m pip install devlogbus
```

For SDK development from a checkout, use
`python3 -m pip install /path/to/DevLogBus/sdk/python`.

```python
from devlogbus import DevLogBusClient, redact_attrs

devlog = DevLogBusClient(
    source="checkout_worker",
    redactor=redact_attrs(["authorization", "token"]),
)

devlog.publish(
    level="INFO",
    message="checkout started",
    attrs={"request_id": "req-1"},
)
```

The default endpoint is `http://127.0.0.1:7423`. Pass `endpoint` explicitly for
other local or trusted-network daemons.

Use the logging handler when you want normal `logging` records to publish:

```python
import logging
from devlogbus import DevLogBusLoggingHandler

logger = logging.getLogger("checkout")
logger.addHandler(DevLogBusLoggingHandler(source="checkout_worker"))
logger.warning("payment provider slow")
```

Run tests from the repository root:

```bash
python3 -m unittest discover -s sdk/python/tests
```
