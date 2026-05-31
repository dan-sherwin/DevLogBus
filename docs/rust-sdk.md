# Rust SDK

The Rust SDK lives in:

```text
sdk/rust
```

It is a small dependency-free publisher for the DevLogBus HTTP API. The default
endpoint is:

```text
http://127.0.0.1:7423
```

## Scope

The Rust SDK includes:

- synchronous HTTP publish
- dependency-free `std::net::TcpStream` transport
- caller-provided `attrs_json`
- filter hook
- redactor hook

It does not include async runtimes, TLS, socket protocol support, a logging
facade adapter, or a JSON object builder.

## Build And Test

```bash
cargo test --manifest-path sdk/rust/Cargo.toml
```

## Client

```rust
use devlogbus::{Client, ClientOptions};

let client = Client::new(ClientOptions {
    source: Some("checkout_worker".to_string()),
    ..Default::default()
});

client.publish_message(
    "INFO",
    "worker started",
    Some("{\"queue\":\"demo\"}".to_string()),
)?;
```

Pass `endpoint` explicitly for a different local or trusted-network daemon:

```rust
let client = Client::new(ClientOptions {
    endpoint: Some("http://devbox:7423".to_string()),
    source: Some("checkout_worker".to_string()),
    ..Default::default()
});
```

## Filters And Redaction

Filters drop records before publishing. Redactors mutate the temporary record
that will be sent to the daemon.

```rust
use devlogbus::{drop_sources, redact_message, Client, ClientOptions};

let client = Client::new(ClientOptions {
    source: Some("checkout_worker".to_string()),
    filter: Some(drop_sources(["noisy_worker"])),
    redactor: Some(redact_message()),
    ..Default::default()
});
```

`attrs_json` must be a JSON object string. The SDK escapes the core record
fields itself but does not parse nested attributes.
