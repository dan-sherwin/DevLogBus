# Java/Kotlin SDK

The Java/Kotlin SDK lives in:

```text
sdk/jvm
```

It is a Java-first JVM SDK that Kotlin can call directly. The default endpoint
is:

```text
http://127.0.0.1:7423
```

## Scope

The JVM SDK includes:

- synchronous HTTP publish through `java.net.http.HttpClient`
- caller-provided `attrsJson`
- filter hook
- redactor hook
- simple logger helper
- Maven Central publishing metadata

It does not include async queues, socket protocol support, Gradle/Maven
build plugins, or framework-specific logging adapters.

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

## Java Client

```java
import com.dansherwin.devlogbus.DevLogBusClient;

DevLogBusClient client = new DevLogBusClient(DevLogBusClient.Options.builder()
        .source("checkout_worker")
        .build());

client.publishMessage("INFO", "worker started", "{\"queue\":\"demo\"}");
```

Pass `endpoint` explicitly for a different local or trusted-network daemon:

```java
DevLogBusClient client = new DevLogBusClient(DevLogBusClient.Options.builder()
        .endpoint("http://devbox:7423")
        .source("checkout_worker")
        .build());
```

## Kotlin Client

```kotlin
import com.dansherwin.devlogbus.DevLogBusClient

val client = DevLogBusClient(
    DevLogBusClient.Options.builder()
        .source("checkout_worker")
        .build()
)

client.publishMessage("INFO", "worker started", "{\"queue\":\"demo\"}")
```

## Filters And Redaction

Filters drop records before publishing. Redactors return the record shape that
will be sent to the daemon.

```java
DevLogBusClient client = new DevLogBusClient(DevLogBusClient.Options.builder()
        .source("checkout_worker")
        .filter(DevLogBusClient.dropSources("noisy_worker"))
        .redactor(DevLogBusClient.redactMessage())
        .build());
```

`attrsJson` must be a JSON object string. The SDK escapes the core record
fields itself but does not parse nested attributes.
