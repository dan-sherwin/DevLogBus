#include "devlogbus.h"

#include <stdio.h>
#include <stdlib.h>

static int keep_record(const devlogbus_record_t *record, void *user_data) {
    (void)user_data;
    return record != NULL && record->message != NULL;
}

int main(void) {
    devlogbus_options_t options = {
        .endpoint = DEVLOGBUS_DEFAULT_HTTP_ENDPOINT,
        .source = "example_c",
        .timeout_ms = 2000,
        .filter = keep_record,
    };

    devlogbus_client_t *client = devlogbus_client_new(&options);
    if (client == NULL) {
        fprintf(stderr, "failed to create DevLogBus client\n");
        return EXIT_FAILURE;
    }

    int rc = devlogbus_publish_message(client, "INFO", "C worker started", "{\"queue\":\"demo\"}");
    if (rc != 0) {
        fprintf(stderr, "%s\n", devlogbus_last_error(client));
        devlogbus_client_free(client);
        return EXIT_FAILURE;
    }

    devlogbus_client_free(client);
    return EXIT_SUCCESS;
}
