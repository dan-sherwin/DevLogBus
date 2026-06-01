# DevLogBus Java/Kotlin SDK

Java-first JVM SDK for sending app records into the DevLogBus live development
stream beside backend, CLI, browser, journal, HTTP, and other SDK records.
Kotlin can call the same classes directly.

Scope:

- synchronous HTTP publish through `java.net.http.HttpClient`
- caller-provided `attrsJson`
- filter hook
- redactor hook
- simple logger helper
- `javac`-verifiable source layout

The default endpoint is:

```text
http://127.0.0.1:7423
```

## Install

Maven:

```xml
<dependency>
  <groupId>io.github.dan-sherwin</groupId>
  <artifactId>devlogbus</artifactId>
  <version>1.3.1</version>
</dependency>
```

Gradle Kotlin DSL:

```kotlin
implementation("io.github.dan-sherwin:devlogbus:1.3.1")
```

Gradle Groovy DSL:

```groovy
implementation 'io.github.dan-sherwin:devlogbus:1.3.1'
```

## Build And Test

```bash
mkdir -p sdk/jvm/build/classes
javac -d sdk/jvm/build/classes \
  $(find sdk/jvm/src/main/java sdk/jvm/src/test/java -name '*.java')
java -cp sdk/jvm/build/classes com.dansherwin.devlogbus.DevLogBusClientTest
```

## Java Use

```java
import com.dansherwin.devlogbus.DevLogBusClient;

DevLogBusClient client = new DevLogBusClient(DevLogBusClient.Options.builder()
        .source("checkout_worker")
        .build());

client.publishMessage("INFO", "worker started", "{\"queue\":\"demo\"}");
```

## Kotlin Use

```kotlin
import com.dansherwin.devlogbus.DevLogBusClient

val client = DevLogBusClient(
    DevLogBusClient.Options.builder()
        .source("checkout_worker")
        .build()
)

client.publishMessage("INFO", "worker started", "{\"queue\":\"demo\"}")
```

`attrsJson` must be a JSON object string. The SDK escapes the core record fields
itself but does not parse nested attributes.
