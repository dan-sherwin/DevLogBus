# DevLogBus Rust SDK

Small Rust SDK for publishing records to the DevLogBus HTTP API.

Scope:

- synchronous HTTP publish
- dependency-free `std::net::TcpStream` transport
- caller-provided `attrs_json`
- filter hook
- redactor hook

The default endpoint is:

```text
http://127.0.0.1:7423
```

## Build And Test

```bash
cargo test --manifest-path sdk/rust/Cargo.toml
```

## Use

```rust
use devlogbus::{Client, ClientOptions};

let client = Client::new(ClientOptions {
    source: Some("checkout_worker".to_string()),
    ..Default::default()
});

client.publish_message("INFO", "worker started", Some("{\"queue\":\"demo\"}".to_string()))?;
```

`attrs_json` must be a JSON object string. The SDK escapes the core record
fields itself but does not parse nested attributes.
