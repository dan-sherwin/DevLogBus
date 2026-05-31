package com.dansherwin.devlogbus;

import java.time.Instant;
import java.util.Objects;

public final class DevLogBusRecord {
    private final String time;
    private final String level;
    private final String source;
    private final String message;
    private final String attrsJson;

    private DevLogBusRecord(Builder builder) {
        this.time = builder.time == null || builder.time.isBlank() ? Instant.now().toString() : builder.time;
        this.level = normalizeLevel(builder.level);
        this.source = Objects.requireNonNullElse(builder.source, "").trim();
        this.message = Objects.requireNonNullElse(builder.message, "");
        this.attrsJson = builder.attrsJson;
        if (this.source.isEmpty()) {
            throw new IllegalArgumentException("DevLogBus source is required");
        }
        if (this.message.isEmpty()) {
            throw new IllegalArgumentException("DevLogBus message is required");
        }
        if (this.attrsJson != null && !this.attrsJson.isBlank() && !this.attrsJson.stripLeading().startsWith("{")) {
            throw new IllegalArgumentException("DevLogBus attrsJson must be a JSON object");
        }
    }

    public String time() {
        return time;
    }

    public String level() {
        return level;
    }

    public String source() {
        return source;
    }

    public String message() {
        return message;
    }

    public String attrsJson() {
        return attrsJson;
    }

    public Builder toBuilder() {
        return builder()
                .time(time)
                .level(level)
                .source(source)
                .message(message)
                .attrsJson(attrsJson);
    }

    public String toJson() {
        StringBuilder out = new StringBuilder();
        out.append('{');
        appendJsonField(out, "time", time);
        out.append(',');
        appendJsonField(out, "level", level);
        out.append(',');
        appendJsonField(out, "source", source);
        out.append(',');
        appendJsonField(out, "message", message);
        if (attrsJson != null && !attrsJson.isBlank()) {
            out.append(",\"attrs\":").append(attrsJson);
        }
        out.append('}');
        return out.toString();
    }

    public static Builder builder() {
        return new Builder();
    }

    public static String normalizeLevel(String level) {
        String value = Objects.requireNonNullElse(level, "").trim();
        return switch (value.toLowerCase()) {
            case "debug", "dbg" -> "DEBUG";
            case "", "info" -> "INFO";
            case "warn", "warning" -> "WARN";
            case "error", "err" -> "ERROR";
            default -> value.toUpperCase();
        };
    }

    private static void appendJsonField(StringBuilder out, String key, String value) {
        out.append('"').append(key).append("\":");
        appendJsonString(out, value);
    }

    private static void appendJsonString(StringBuilder out, String value) {
        out.append('"');
        for (int i = 0; i < value.length(); i++) {
            char ch = value.charAt(i);
            switch (ch) {
                case '"' -> out.append("\\\"");
                case '\\' -> out.append("\\\\");
                case '\b' -> out.append("\\b");
                case '\f' -> out.append("\\f");
                case '\n' -> out.append("\\n");
                case '\r' -> out.append("\\r");
                case '\t' -> out.append("\\t");
                default -> {
                    if (ch < 0x20) {
                        out.append(String.format("\\u%04x", (int) ch));
                    } else {
                        out.append(ch);
                    }
                }
            }
        }
        out.append('"');
    }

    public static final class Builder {
        private String time;
        private String level = "INFO";
        private String source;
        private String message;
        private String attrsJson;

        public Builder time(String time) {
            this.time = time;
            return this;
        }

        public Builder level(String level) {
            this.level = level;
            return this;
        }

        public Builder source(String source) {
            this.source = source;
            return this;
        }

        public Builder message(String message) {
            this.message = message;
            return this;
        }

        public Builder attrsJson(String attrsJson) {
            this.attrsJson = attrsJson;
            return this;
        }

        public DevLogBusRecord build() {
            return new DevLogBusRecord(this);
        }
    }
}
