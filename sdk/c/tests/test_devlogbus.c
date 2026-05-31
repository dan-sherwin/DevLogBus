#include "devlogbus.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static int failures = 0;

static void expect(int ok, const char *message) {
    if (!ok) {
        fprintf(stderr, "FAIL: %s\n", message);
        failures++;
    }
}

static int drop_all(const devlogbus_record_t *record, void *user_data) {
    (void)record;
    (void)user_data;
    return 0;
}

static int redact_message(devlogbus_record_t *record, void *user_data) {
    (void)user_data;
    record->message = DEVLOGBUS_REDACTED_VALUE;
    return 0;
}

int main(void) {
    devlogbus_record_t record;
    char *json;
    devlogbus_client_t *client;
    devlogbus_options_t options;
    int rc;

    memset(&record, 0, sizeof(record));
    record.time = "2026-05-31T12:00:00Z";
    record.level = "warning";
    record.source = "c_test";
    record.message = "quote \" and newline\n";
    record.attrs_json = "{\"request\":{\"id\":\"req-1\"}}";

    json = devlogbus_record_json(&record);
    expect(json != NULL, "record JSON should be built");
    if (json != NULL) {
        expect(strstr(json, "\"level\":\"WARN\"") != NULL, "level should normalize to WARN");
        expect(strstr(json, "\"source\":\"c_test\"") != NULL, "source should be encoded");
        expect(strstr(json, "quote \\\" and newline\\n") != NULL, "message should be JSON escaped");
        expect(strstr(json, "\"attrs\":{\"request\":{\"id\":\"req-1\"}}") != NULL, "attrs JSON should be embedded");
        devlogbus_free_string(json);
    }

    record.attrs_json = "\"not an object\"";
    json = devlogbus_record_json(&record);
    expect(json == NULL, "attrs_json should require a JSON object");

    memset(&options, 0, sizeof(options));
    options.endpoint = "http://127.0.0.1:1";
    options.source = "filtered";
    options.filter = drop_all;
    client = devlogbus_client_new(&options);
    expect(client != NULL, "client should be created");
    if (client != NULL) {
        rc = devlogbus_publish_message(client, "INFO", "filtered out", NULL);
        expect(rc == 0, "filtered records should not require HTTP");
        devlogbus_client_free(client);
    }

    memset(&options, 0, sizeof(options));
    options.endpoint = "http://127.0.0.1:1";
    options.source = "redacted";
    options.redactor = redact_message;
    client = devlogbus_client_new(&options);
    expect(client != NULL, "redactor client should be created");
    if (client != NULL) {
        memset(&record, 0, sizeof(record));
        record.source = "redacted";
        record.level = "INFO";
        record.message = "secret";
        rc = options.redactor(&record, NULL);
        expect(rc == 0, "redactor callback should succeed");
        json = devlogbus_record_json(&record);
        expect(json != NULL && strstr(json, DEVLOGBUS_REDACTED_VALUE) != NULL, "redactor should update record fields");
        devlogbus_free_string(json);
        devlogbus_client_free(client);
    }

    return failures == 0 ? EXIT_SUCCESS : EXIT_FAILURE;
}
