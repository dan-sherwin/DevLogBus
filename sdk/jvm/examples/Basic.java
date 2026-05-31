import com.dansherwin.devlogbus.DevLogBusClient;

public final class Basic {
    public static void main(String[] args) throws Exception {
        DevLogBusClient client = new DevLogBusClient(DevLogBusClient.Options.builder()
                .source("example_jvm")
                .build());

        client.publishMessage("INFO", "JVM worker started", "{\"queue\":\"demo\"}");
    }
}
