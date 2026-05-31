# DevLogBus Browser Tap Privacy Policy

Effective date: May 31, 2026

DevLogBus Browser Tap is a Chrome extension for local development workflows. It
publishes user-selected browser debugging events to a DevLogBus daemon endpoint
configured by the user.

## Single Purpose

The extension's single purpose is to publish user-triggered active-tab
debugging events to the user's local DevLogBus daemon.

## Data The Extension Handles

Browser Tap only captures data after the user clicks Attach for the active tab.
Depending on enabled capture options, records may include:

- tab title
- tab URL
- console messages
- runtime exception messages
- browser log messages
- network request URLs
- response status codes
- response MIME types
- request failure messages

The extension does not intentionally collect personal communications, financial
information, health information, location, or personally identifiable
information. However, debugging text and URLs can contain sensitive data if a
website or application includes it in console output, exception text, or network
URLs.

## Where Data Is Sent

Captured records are posted to the configured DevLogBus daemon endpoint. The
default endpoint is:

```text
http://127.0.0.1:7423
```

DevLogBus does not operate a cloud collection service for this extension.
Captured records are not sent to DevLogBus-owned servers.

## Local Storage

The extension stores configuration in Chrome local extension storage, including:

- daemon endpoint
- source override
- capture toggles
- allow and deny patterns
- redaction toggles

The extension does not store captured debugging records in Chrome extension
storage.

## Redaction

Browser Tap redacts common sensitive values before publishing records when
redaction options are enabled. Redaction covers common authorization headers,
bearer/basic credentials, API keys, sensitive query parameters, and cookie
values.

Redaction is a safety net, not a guarantee. Users should avoid attaching the
extension to sensitive workflows or pages that may expose secrets in unusual
formats.

## Data Use

Data handled by the extension is used only for extension functionality:
publishing selected browser debugging events into the user's DevLogBus workflow.

The extension does not use captured data for:

- advertising
- analytics
- personalization
- sale or transfer to third parties

## User Control

Users control when capture starts and stops. Capture starts only after clicking
Attach and stops when the user clicks Detach, closes the tab, disables capture,
or Chrome ends the debugger session.

Users can restrict capture with allow and deny patterns before attaching to a
tab.

## Contact

Report privacy or security concerns through the DevLogBus project repository:

```text
https://github.com/dan-sherwin/DevLogBus
```
