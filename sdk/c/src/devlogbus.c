#include "devlogbus.h"

#include <ctype.h>
#include <curl/curl.h>
#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

struct devlogbus_client {
    char *endpoint;
    char *source;
    long timeout_ms;
    devlogbus_record_filter_fn filter;
    void *filter_user_data;
    devlogbus_record_redactor_fn redactor;
    void *redactor_user_data;
    char last_error[256];
};

typedef struct devlogbus_buffer {
    char *data;
    size_t len;
    size_t cap;
} devlogbus_buffer_t;

static size_t curl_clients = 0;

static int append_char(devlogbus_buffer_t *buf, char value);
static int append_text(devlogbus_buffer_t *buf, const char *value);
static int append_json_string(devlogbus_buffer_t *buf, const char *value);
static int attrs_json_is_object(const char *attrs_json);
static char *dup_string(const char *value);
static char *endpoint_url(const char *endpoint);
static void set_error(devlogbus_client_t *client, const char *format, ...);
static char *normalize_endpoint(const char *endpoint);
static char *normalize_level(const char *level);
static int current_time_iso(char *buf, size_t size);

devlogbus_client_t *devlogbus_client_new(const devlogbus_options_t *options) {
    if (curl_clients == 0 && curl_global_init(CURL_GLOBAL_DEFAULT) != 0) {
        return NULL;
    }
    curl_clients++;

    devlogbus_client_t *client = (devlogbus_client_t *)calloc(1, sizeof(devlogbus_client_t));
    if (client == NULL) {
        curl_clients--;
        if (curl_clients == 0) {
            curl_global_cleanup();
        }
        return NULL;
    }

    const char *endpoint = DEVLOGBUS_DEFAULT_HTTP_ENDPOINT;
    if (options != NULL && options->endpoint != NULL && options->endpoint[0] != '\0') {
        endpoint = options->endpoint;
    }

    client->endpoint = normalize_endpoint(endpoint);
    client->source = dup_string(options != NULL && options->source != NULL ? options->source : "");
    client->timeout_ms = options != NULL && options->timeout_ms > 0 ? options->timeout_ms : 2000;
    client->filter = options != NULL ? options->filter : NULL;
    client->filter_user_data = options != NULL ? options->filter_user_data : NULL;
    client->redactor = options != NULL ? options->redactor : NULL;
    client->redactor_user_data = options != NULL ? options->redactor_user_data : NULL;

    if (client->endpoint == NULL || client->source == NULL) {
        devlogbus_client_free(client);
        return NULL;
    }
    return client;
}

void devlogbus_client_free(devlogbus_client_t *client) {
    if (client == NULL) {
        return;
    }
    free(client->endpoint);
    free(client->source);
    free(client);

    if (curl_clients > 0) {
        curl_clients--;
        if (curl_clients == 0) {
            curl_global_cleanup();
        }
    }
}

int devlogbus_publish(devlogbus_client_t *client, const devlogbus_record_t *record) {
    if (client == NULL || record == NULL) {
        return -1;
    }

    devlogbus_record_t prepared = *record;
    if ((prepared.source == NULL || prepared.source[0] == '\0') && client->source != NULL) {
        prepared.source = client->source;
    }

    if (client->filter != NULL && !client->filter(&prepared, client->filter_user_data)) {
        return 0;
    }
    if (client->redactor != NULL && client->redactor(&prepared, client->redactor_user_data) != 0) {
        set_error(client, "DevLogBus redactor failed");
        return -1;
    }

    char *payload = devlogbus_record_json(&prepared);
    if (payload == NULL) {
        set_error(client, "DevLogBus record is invalid");
        return -1;
    }

    char *url = endpoint_url(client->endpoint);
    if (url == NULL) {
        free(payload);
        set_error(client, "DevLogBus endpoint allocation failed");
        return -1;
    }

    CURL *curl = curl_easy_init();
    if (curl == NULL) {
        free(payload);
        free(url);
        set_error(client, "curl_easy_init failed");
        return -1;
    }

    struct curl_slist *headers = NULL;
    headers = curl_slist_append(headers, "Content-Type: application/json");

    curl_easy_setopt(curl, CURLOPT_URL, url);
    curl_easy_setopt(curl, CURLOPT_POST, 1L);
    curl_easy_setopt(curl, CURLOPT_POSTFIELDS, payload);
    curl_easy_setopt(curl, CURLOPT_POSTFIELDSIZE, (long)strlen(payload));
    curl_easy_setopt(curl, CURLOPT_HTTPHEADER, headers);
    curl_easy_setopt(curl, CURLOPT_TIMEOUT_MS, client->timeout_ms);
    curl_easy_setopt(curl, CURLOPT_NOSIGNAL, 1L);

    CURLcode res = curl_easy_perform(curl);
    long response_code = 0;
    if (res == CURLE_OK) {
        curl_easy_getinfo(curl, CURLINFO_RESPONSE_CODE, &response_code);
    }

    curl_slist_free_all(headers);
    curl_easy_cleanup(curl);
    free(payload);
    free(url);

    if (res != CURLE_OK) {
        set_error(client, "DevLogBus publish failed: %s", curl_easy_strerror(res));
        return -1;
    }
    if (response_code < 200 || response_code >= 300) {
        set_error(client, "DevLogBus publish failed: HTTP %ld", response_code);
        return -1;
    }

    client->last_error[0] = '\0';
    return 0;
}

int devlogbus_publish_message(devlogbus_client_t *client, const char *level, const char *message, const char *attrs_json) {
    devlogbus_record_t record;
    memset(&record, 0, sizeof(record));
    record.level = level;
    record.message = message;
    record.attrs_json = attrs_json;
    return devlogbus_publish(client, &record);
}

const char *devlogbus_last_error(const devlogbus_client_t *client) {
    if (client == NULL) {
        return "";
    }
    return client->last_error;
}

char *devlogbus_record_json(const devlogbus_record_t *record) {
    if (record == NULL || record->source == NULL || record->source[0] == '\0' || record->message == NULL || record->message[0] == '\0') {
        return NULL;
    }
    if (record->attrs_json != NULL && record->attrs_json[0] != '\0' && !attrs_json_is_object(record->attrs_json)) {
        return NULL;
    }

    char time_buf[32];
    const char *record_time = record->time;
    if (record_time == NULL || record_time[0] == '\0') {
        if (current_time_iso(time_buf, sizeof(time_buf)) != 0) {
            return NULL;
        }
        record_time = time_buf;
    }

    char *level = normalize_level(record->level);
    if (level == NULL) {
        return NULL;
    }

    devlogbus_buffer_t buf;
    memset(&buf, 0, sizeof(buf));
    if (!append_char(&buf, '{') ||
        !append_text(&buf, "\"time\":") || !append_json_string(&buf, record_time) ||
        !append_text(&buf, ",\"level\":") || !append_json_string(&buf, level) ||
        !append_text(&buf, ",\"source\":") || !append_json_string(&buf, record->source) ||
        !append_text(&buf, ",\"message\":") || !append_json_string(&buf, record->message)) {
        free(level);
        free(buf.data);
        return NULL;
    }
    free(level);

    if (record->attrs_json != NULL && record->attrs_json[0] != '\0') {
        if (!append_text(&buf, ",\"attrs\":") || !append_text(&buf, record->attrs_json)) {
            free(buf.data);
            return NULL;
        }
    }

    if (!append_char(&buf, '}') || !append_char(&buf, '\0')) {
        free(buf.data);
        return NULL;
    }
    return buf.data;
}

void devlogbus_free_string(char *value) {
    free(value);
}

static int append_char(devlogbus_buffer_t *buf, char value) {
    if (buf->len + 1 >= buf->cap) {
        size_t next = buf->cap == 0 ? 128 : buf->cap * 2;
        char *data = (char *)realloc(buf->data, next);
        if (data == NULL) {
            return 0;
        }
        buf->data = data;
        buf->cap = next;
    }
    buf->data[buf->len++] = value;
    return 1;
}

static int append_text(devlogbus_buffer_t *buf, const char *value) {
    if (value == NULL) {
        value = "";
    }
    while (*value != '\0') {
        if (!append_char(buf, *value++)) {
            return 0;
        }
    }
    return 1;
}

static int append_json_string(devlogbus_buffer_t *buf, const char *value) {
    static const char hex[] = "0123456789abcdef";
    if (!append_char(buf, '"')) {
        return 0;
    }
    if (value == NULL) {
        value = "";
    }
    while (*value != '\0') {
        unsigned char ch = (unsigned char)*value++;
        switch (ch) {
        case '"':
            if (!append_text(buf, "\\\"")) {
                return 0;
            }
            break;
        case '\\':
            if (!append_text(buf, "\\\\")) {
                return 0;
            }
            break;
        case '\b':
            if (!append_text(buf, "\\b")) {
                return 0;
            }
            break;
        case '\f':
            if (!append_text(buf, "\\f")) {
                return 0;
            }
            break;
        case '\n':
            if (!append_text(buf, "\\n")) {
                return 0;
            }
            break;
        case '\r':
            if (!append_text(buf, "\\r")) {
                return 0;
            }
            break;
        case '\t':
            if (!append_text(buf, "\\t")) {
                return 0;
            }
            break;
        default:
            if (ch < 0x20) {
                if (!append_text(buf, "\\u00") || !append_char(buf, hex[ch >> 4]) || !append_char(buf, hex[ch & 0x0f])) {
                    return 0;
                }
            } else if (!append_char(buf, (char)ch)) {
                return 0;
            }
            break;
        }
    }
    return append_char(buf, '"');
}

static int attrs_json_is_object(const char *attrs_json) {
    while (*attrs_json != '\0' && isspace((unsigned char)*attrs_json)) {
        attrs_json++;
    }
    return *attrs_json == '{';
}

static char *dup_string(const char *value) {
    size_t len;
    char *out;
    if (value == NULL) {
        value = "";
    }
    len = strlen(value);
    out = (char *)malloc(len + 1);
    if (out == NULL) {
        return NULL;
    }
    memcpy(out, value, len + 1);
    return out;
}

static char *endpoint_url(const char *endpoint) {
    const char *path = "/api/records";
    size_t len = strlen(endpoint) + strlen(path) + 1;
    char *url = (char *)malloc(len);
    if (url == NULL) {
        return NULL;
    }
    snprintf(url, len, "%s%s", endpoint, path);
    return url;
}

static void set_error(devlogbus_client_t *client, const char *format, ...) {
    if (client == NULL) {
        return;
    }
    va_list args;
    va_start(args, format);
    vsnprintf(client->last_error, sizeof(client->last_error), format, args);
    va_end(args);
}

static char *normalize_endpoint(const char *endpoint) {
    char *out = dup_string(endpoint != NULL && endpoint[0] != '\0' ? endpoint : DEVLOGBUS_DEFAULT_HTTP_ENDPOINT);
    if (out == NULL) {
        return NULL;
    }
    size_t len = strlen(out);
    while (len > 0 && out[len - 1] == '/') {
        out[--len] = '\0';
    }
    return out;
}

static char *normalize_level(const char *level) {
    char *out;
    size_t start = 0;
    size_t end;
    size_t len;
    size_t i;

    if (level == NULL) {
        level = "INFO";
    }
    end = strlen(level);
    while (isspace((unsigned char)level[start])) {
        start++;
    }
    while (end > start && isspace((unsigned char)level[end - 1])) {
        end--;
    }
    len = end - start;
    out = (char *)malloc(len + 1);
    if (out == NULL) {
        return NULL;
    }
    for (i = 0; i < len; i++) {
        out[i] = (char)toupper((unsigned char)level[start + i]);
    }
    out[len] = '\0';

    if (strcmp(out, "") == 0) {
        free(out);
        return dup_string("INFO");
    }
    if (strcmp(out, "DBG") == 0) {
        free(out);
        return dup_string("DEBUG");
    }
    if (strcmp(out, "WARNING") == 0) {
        free(out);
        return dup_string("WARN");
    }
    if (strcmp(out, "ERR") == 0) {
        free(out);
        return dup_string("ERROR");
    }
    return out;
}

static int current_time_iso(char *buf, size_t size) {
    time_t now = time(NULL);
    struct tm tm_utc;
    if (now == (time_t)-1) {
        return -1;
    }
#ifdef _WIN32
    if (gmtime_s(&tm_utc, &now) != 0) {
        return -1;
    }
#else
    if (gmtime_r(&now, &tm_utc) == NULL) {
        return -1;
    }
#endif
    return strftime(buf, size, "%Y-%m-%dT%H:%M:%SZ", &tm_utc) == 0 ? -1 : 0;
}
