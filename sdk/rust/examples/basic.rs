use devlogbus::{Client, ClientOptions};

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let client = Client::new(ClientOptions {
        source: Some("example_rust".to_string()),
        ..Default::default()
    });

    client.publish_message(
        "INFO",
        "Rust worker started",
        Some("{\"queue\":\"demo\"}".to_string()),
    )?;
    Ok(())
}
