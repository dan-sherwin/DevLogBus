package com.dansherwin.devlogbus;

import java.io.IOException;

public final class DevLogBusLogger {
    private final DevLogBusClient client;

    DevLogBusLogger(DevLogBusClient client) {
        this.client = client;
    }

    public DevLogBusClient.PublishResult debug(String message, String attrsJson) throws IOException, InterruptedException {
        return client.publishMessage("DEBUG", message, attrsJson);
    }

    public DevLogBusClient.PublishResult info(String message, String attrsJson) throws IOException, InterruptedException {
        return client.publishMessage("INFO", message, attrsJson);
    }

    public DevLogBusClient.PublishResult warn(String message, String attrsJson) throws IOException, InterruptedException {
        return client.publishMessage("WARN", message, attrsJson);
    }

    public DevLogBusClient.PublishResult error(String message, String attrsJson) throws IOException, InterruptedException {
        return client.publishMessage("ERROR", message, attrsJson);
    }
}
