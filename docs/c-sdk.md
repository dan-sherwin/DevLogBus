# C SDK

The C SDK lives in:

```text
sdk/c
```

It is a deliberately small HTTP publisher for native tools and services. The
default endpoint is:

```text
http://127.0.0.1:7423
```

## Scope

The C SDK includes:

- synchronous publish
- `libcurl` transport
- CMake build
- caller-provided `attrs_json`
- filter callback
- redactor callback

It does not include async queues, socket protocol support, a logging framework,
custom allocators, or a JSON object builder.

## Build

```bash
cmake -S sdk/c -B sdk/c/build
cmake --build sdk/c/build
ctest --test-dir sdk/c/build --output-on-failure
```

## Client

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
fields itself but does not parse nested attributes.

## Filters And Redaction

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

For structured attr redaction, build the already-redacted `attrs_json` before
calling the SDK.
