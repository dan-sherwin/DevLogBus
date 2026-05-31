package com.dansherwin.devlogbus;

import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.util.HashSet;
import java.util.Objects;
import java.util.Set;

public final class DevLogBusClient {
    public static final String DEFAULT_HTTP_ENDPOINT = "http://127.0.0.1:7423";
    public static final String REDACTED_VALUE = "[REDACTED]";

    private final String endpoint;
    private final String source;
    private final Duration timeout;
    private final RecordFilter filter;
    private final RecordRedactor redactor;
    private final HttpClient httpClient;

    public DevLogBusClient(Options options) {
        Options actual = options == null ? Options.builder().build() : options;
        this.endpoint = trimEndpoint(actual.endpoint);
        this.source = Objects.requireNonNullElse(actual.source, "");
        this.timeout = actual.timeout == null ? Duration.ofSeconds(2) : actual.timeout;
        this.filter = actual.filter;
        this.redactor = actual.redactor;
        this.httpClient = actual.httpClient == null ? HttpClient.newHttpClient() : actual.httpClient;
    }

    public PublishResult publishMessage(String level, String message, String attrsJson) throws IOException, InterruptedException {
        return publish(DevLogBusRecord.builder()
                .source(source)
                .level(level)
                .message(message)
                .attrsJson(attrsJson)
                .build());
    }

    public PublishResult publish(DevLogBusRecord input) throws IOException, InterruptedException {
        DevLogBusRecord record = input.source().isBlank()
                ? input.toBuilder().source(source).build()
                : input;
        if (filter != null && !filter.shouldPublish(record)) {
            return new PublishResult(0, true);
        }
        if (redactor != null) {
            record = redactor.redact(record);
        }

        HttpRequest request = HttpRequest.newBuilder(URI.create(endpoint + "/api/records"))
                .timeout(timeout)
                .header("Content-Type", "application/json")
                .POST(HttpRequest.BodyPublishers.ofString(record.toJson()))
                .build();
        HttpResponse<String> response = httpClient.send(request, HttpResponse.BodyHandlers.ofString());
        if (response.statusCode() < 200 || response.statusCode() >= 300) {
            throw new IOException("DevLogBus publish failed: HTTP " + response.statusCode());
        }
        return new PublishResult(1, false);
    }

    public DevLogBusLogger logger() {
        return new DevLogBusLogger(this);
    }

    public static RecordFilter dropSources(String... sources) {
        Set<String> blocked = new HashSet<>();
        for (String source : sources) {
            if (source != null && !source.isBlank()) {
                blocked.add(source.trim());
            }
        }
        return record -> !blocked.contains(record.source());
    }

    public static RecordRedactor redactMessage() {
        return record -> record.toBuilder().message(REDACTED_VALUE).build();
    }

    private static String trimEndpoint(String endpoint) {
        String value = Objects.requireNonNullElse(endpoint, DEFAULT_HTTP_ENDPOINT).trim();
        while (value.endsWith("/")) {
            value = value.substring(0, value.length() - 1);
        }
        return value.isEmpty() ? DEFAULT_HTTP_ENDPOINT : value;
    }

    public record PublishResult(int published, boolean filtered) {
    }

    public static final class Options {
        private String endpoint = DEFAULT_HTTP_ENDPOINT;
        private String source = "";
        private Duration timeout = Duration.ofSeconds(2);
        private RecordFilter filter;
        private RecordRedactor redactor;
        private HttpClient httpClient;

        private Options() {
        }

        public static Builder builder() {
            return new Builder();
        }

        public static final class Builder {
            private final Options options = new Options();

            public Builder endpoint(String endpoint) {
                options.endpoint = endpoint;
                return this;
            }

            public Builder source(String source) {
                options.source = source;
                return this;
            }

            public Builder timeout(Duration timeout) {
                options.timeout = timeout;
                return this;
            }

            public Builder filter(RecordFilter filter) {
                options.filter = filter;
                return this;
            }

            public Builder redactor(RecordRedactor redactor) {
                options.redactor = redactor;
                return this;
            }

            public Builder httpClient(HttpClient httpClient) {
                options.httpClient = httpClient;
                return this;
            }

            public Options build() {
                return options;
            }
        }
    }
}
