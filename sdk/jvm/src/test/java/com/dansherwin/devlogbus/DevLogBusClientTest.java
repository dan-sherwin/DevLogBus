package com.dansherwin.devlogbus;

import com.sun.net.httpserver.HttpServer;

import java.io.IOException;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.List;

public final class DevLogBusClientTest {
    private static final List<String> requests = new ArrayList<>();

    public static void main(String[] args) throws Exception {
        normalizesLevels();
        buildsRecordJson();
        rejectsNonObjectAttrsJson();
        filtersBeforeHttpPublish();
        publishesToHttpEndpoint();
    }

    private static void normalizesLevels() {
        assertEquals("WARN", DevLogBusRecord.normalizeLevel("warning"), "warning should normalize");
        assertEquals("DEBUG", DevLogBusRecord.normalizeLevel("dbg"), "dbg should normalize");
        assertEquals("CUSTOM", DevLogBusRecord.normalizeLevel("custom"), "custom should uppercase");
    }

    private static void buildsRecordJson() {
        String json = DevLogBusRecord.builder()
                .time("2026-05-31T12:00:00Z")
                .source("jvm_test")
                .level("warning")
                .message("quote \" newline\n")
                .attrsJson("{\"request\":{\"id\":\"req-1\"}}")
                .build()
                .toJson();

        assertTrue(json.contains("\"level\":\"WARN\""), "level should be WARN");
        assertTrue(json.contains("\"source\":\"jvm_test\""), "source should be encoded");
        assertTrue(json.contains("quote \\\" newline\\n"), "message should be escaped");
        assertTrue(json.contains("\"attrs\":{\"request\":{\"id\":\"req-1\"}}"), "attrs should be embedded");
    }

    private static void rejectsNonObjectAttrsJson() {
        assertThrows(() -> DevLogBusRecord.builder()
                .source("jvm_test")
                .message("hello")
                .attrsJson("\"nope\"")
                .build(), "attrsJson should require object JSON");
    }

    private static void filtersBeforeHttpPublish() throws Exception {
        DevLogBusClient client = new DevLogBusClient(DevLogBusClient.Options.builder()
                .endpoint("http://127.0.0.1:1")
                .source("hidden")
                .filter(DevLogBusClient.dropSources("hidden"))
                .build());

        DevLogBusClient.PublishResult result = client.publishMessage("INFO", "drop me", null);

        assertEquals(0, result.published(), "filtered publish count");
        assertTrue(result.filtered(), "result should be filtered");
    }

    private static void publishesToHttpEndpoint() throws Exception {
        requests.clear();
        HttpServer server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
        server.createContext("/api/records", exchange -> {
            requests.add(new String(exchange.getRequestBody().readAllBytes(), StandardCharsets.UTF_8));
            byte[] body = "{\"published\":1}".getBytes(StandardCharsets.UTF_8);
            exchange.sendResponseHeaders(200, body.length);
            exchange.getResponseBody().write(body);
            exchange.close();
        });
        server.start();
        try {
            DevLogBusClient client = new DevLogBusClient(DevLogBusClient.Options.builder()
                    .endpoint("http://127.0.0.1:" + server.getAddress().getPort())
                    .source("jvm_test")
                    .redactor(DevLogBusClient.redactMessage())
                    .build());

            DevLogBusClient.PublishResult result = client.publishMessage("INFO", "secret", null);

            assertEquals(1, result.published(), "published count");
            assertTrue(!result.filtered(), "result should not be filtered");
            assertTrue(requests.get(0).contains(DevLogBusClient.REDACTED_VALUE), "request should contain redacted value");
        } finally {
            server.stop(0);
        }
    }

    private static void assertEquals(Object want, Object got, String message) {
        if (!want.equals(got)) {
            throw new AssertionError(message + ": got " + got + ", want " + want);
        }
    }

    private static void assertTrue(boolean ok, String message) {
        if (!ok) {
            throw new AssertionError(message);
        }
    }

    private static void assertThrows(ThrowingRunnable runnable, String message) {
        try {
            runnable.run();
        } catch (RuntimeException expected) {
            return;
        } catch (Exception unexpected) {
            throw new AssertionError(message + ": unexpected checked exception " + unexpected);
        }
        throw new AssertionError(message + ": expected exception");
    }

    @FunctionalInterface
    private interface ThrowingRunnable {
        void run() throws Exception;
    }
}
