# SDK Install

DevLogBus SDKs feed application records into the same live development stream as
backend services, CLI tools, Browser Tap events, Linux `journald`, and direct
HTTP records. They publish to the daemon HTTP API at:

```text
http://127.0.0.1:7423
```

Start `devlogbusd` first, then install the SDK that matches the application you
want to instrument.

## Go

```bash
go get github.com/dan-sherwin/devlogbus@v1.3.1
```

Use the Go packages directly:

```text
github.com/dan-sherwin/devlogbus/pkg/client
github.com/dan-sherwin/devlogbus/pkg/protocol
github.com/dan-sherwin/devlogbus/pkg/runtime
github.com/dan-sherwin/devlogbus/pkg/sloghandler
```

## C

The C SDK is source-distributed in the repository and release source archives.
It is intentionally small and builds with CMake and `libcurl`:

```bash
cmake -S sdk/c -B sdk/c/build
cmake --build sdk/c/build
```

## .NET / C#

```bash
dotnet add package DanSherwin.DevLogBus.Sdk
```

## Rust

```bash
cargo add devlogbus
```

## Java / Kotlin

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

## Node / TypeScript

```bash
npm install @dan-sherwin/devlogbus
```

## Python

```bash
python3 -m pip install devlogbus
```

## Source Checkout Installs

Each SDK also remains usable from a source checkout for local development,
testing, or patch work. See the language-specific SDK pages for examples,
filters, redaction hooks, and local test commands.
