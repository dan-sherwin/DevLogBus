# DevLogBus C SDK

Small C SDK for publishing records to the DevLogBus HTTP API.

Scope:

- synchronous HTTP publish
- `libcurl` transport
- caller-provided `attrs_json`
- filter callback
- redactor callback
- CMake build

The default endpoint is:

```text
http://127.0.0.1:7423
```

## Build

```bash
cmake -S sdk/c -B sdk/c/build
cmake --build sdk/c/build
ctest --test-dir sdk/c/build --output-on-failure
```

## Use

```c
#include "devlogbus.h"

devlogbus_options_t options = {
    .endpoint = DEVLOGBUS_DEFAULT_HTTP_ENDPOINT,
    .source = "checkout_native",
    .timeout_ms = 2000,
};

devlogbus_client_t *client = devlogbus_client_new(&options);
devlogbus_publish_message(client, "INFO", "worker started", "{\"queue\":\"demo\"}");
devlogbus_client_free(client);
```

`attrs_json` must be a JSON object string. The SDK escapes the core record
fields itself but does not parse or validate nested attributes beyond requiring
the attrs string to start with `{`.

## Hooks

Filters return non-zero to publish a record:

```c
static int keep_record(const devlogbus_record_t *record, void *user_data) {
    (void)user_data;
    return record != NULL && record->source != NULL;
}
```

Redactors may update pointer fields on the temporary record passed to the hook:

```c
static int redact_message(devlogbus_record_t *record, void *user_data) {
    (void)user_data;
    record->message = DEVLOGBUS_REDACTED_VALUE;
    return 0;
}
```

The C SDK is intentionally not a full logging framework integration.
