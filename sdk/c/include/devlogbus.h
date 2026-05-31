#ifndef DEVLOGBUS_H
#define DEVLOGBUS_H

#ifdef __cplusplus
extern "C" {
#endif

#define DEVLOGBUS_DEFAULT_HTTP_ENDPOINT "http://127.0.0.1:7423"
#define DEVLOGBUS_REDACTED_VALUE "[REDACTED]"

typedef struct devlogbus_client devlogbus_client_t;

typedef struct devlogbus_record {
    const char *time;
    const char *level;
    const char *source;
    const char *message;
    const char *attrs_json;
} devlogbus_record_t;

typedef int (*devlogbus_record_filter_fn)(const devlogbus_record_t *record, void *user_data);
typedef int (*devlogbus_record_redactor_fn)(devlogbus_record_t *record, void *user_data);

typedef struct devlogbus_options {
    const char *endpoint;
    const char *source;
    long timeout_ms;
    devlogbus_record_filter_fn filter;
    void *filter_user_data;
    devlogbus_record_redactor_fn redactor;
    void *redactor_user_data;
} devlogbus_options_t;

devlogbus_client_t *devlogbus_client_new(const devlogbus_options_t *options);
void devlogbus_client_free(devlogbus_client_t *client);

int devlogbus_publish(devlogbus_client_t *client, const devlogbus_record_t *record);
int devlogbus_publish_message(devlogbus_client_t *client, const char *level, const char *message, const char *attrs_json);

const char *devlogbus_last_error(const devlogbus_client_t *client);

char *devlogbus_record_json(const devlogbus_record_t *record);
void devlogbus_free_string(char *value);

#ifdef __cplusplus
}
#endif

#endif
