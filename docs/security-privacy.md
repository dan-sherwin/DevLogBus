# Security And Privacy

DevLogBus is local-first by design. It is intended for a single developer,
trusted workstation, or explicitly trusted private network during active
debugging.

## No Cloud Service

DevLogBus does not operate a hosted collection service. The daemon, UI, CLI, Go
SDK, journal bridge, and Browser Tap all publish to an endpoint you control.

The default HTTP endpoint is:

```text
http://127.0.0.1:7423
```

The default Go/CLI endpoint is:

- macOS/Linux: `/tmp/devlogbus/devlogbus.sock`
- Windows: `127.0.0.1:7422`

## Optional UI Login

DevLogBus is open by default. A fresh daemon install has no users and does not
require a login.

The embedded browser UI can enable login mode from Settings after at least one
user has been added. UI users have a username, display name, and password. When
login mode is enabled, the browser UI requires a session cookie and protects the
HTTP APIs that expose buffered records and daemon details.

The Go/CLI socket endpoint remains a local developer endpoint. Do not bind
DevLogBus listeners to public interfaces unless the network itself is trusted.

Safe workstation default:

```bash
devlogbusd run
```

Trusted private-network troubleshooting only:

```bash
devlogbusd run --endpoint 0.0.0.0:7422 --http 0.0.0.0:7423
```

Never expose those listeners directly to the internet.

## Verification And User Choice

DevLogBus publishes checksums, package signing keys, and signed package metadata
where practical. The project also documents the commands for users who want
APT, RPM, or Alpine package signature checks.

The default package install docs choose low-friction local developer setup. They
do not force every operator through verification ceremony just to get a tool
running on their own machine.

DevLogBus provides the tools you need to maintain your own security, but it does
not force you to use them. The project publishes checksums, signing keys, and
verification instructions. Use them as you see fit, because I am not your mother
and it is not my job to make sure you wear a damn helmet. That choice belongs to
you.

In short, piss on the electric fence if you want. Just don't act surprised when
physics files a bug report on your ass.

## Browser Tap Capture Surface

Browser Tap captures only after the user clicks Attach for the active tab.
Depending on enabled toggles, it can publish:

- console calls
- runtime exceptions
- browser log entries
- network request URLs
- response status codes
- request failure text
- tab title and tab URL

It posts records to the configured daemon HTTP endpoint. The Chrome Web Store
package grants host access only for localhost daemon endpoints:

```text
http://127.0.0.1/*
http://localhost/*
```

## Chrome Debugger Permission

Browser Tap uses Chrome's `debugger` permission so it can receive console,
runtime, log, and network events from the active tab after an explicit attach.
Chrome displays its debugger warning while the extension is attached.

If DevTools attaches to the same tab or the debugger session is otherwise
detached, Browser Tap publishes a warning record so the DevLogBus stream makes
the loss of browser capture visible.

## Redaction

Browser Tap redacts common sensitive values before publishing:

- authorization headers
- bearer/basic credentials
- API keys
- common sensitive URL parameters
- cookie values

Redaction is a safety net, not a guarantee. Console messages, exception text,
URLs, and custom attributes can still contain secrets if an application logs
them in unusual shapes.

The Go, C, .NET/C#, Rust, Java/Kotlin, Node/TypeScript, and Python SDKs also
expose record filter and redaction hooks. Use those hooks at application
boundaries to drop noisy records or redact known sensitive attributes before
they leave the process.

## Safe Usage Recommendations

- Keep listeners on loopback unless actively troubleshooting across trusted
  machines.
- Do not publish production secrets, credentials, personal data, or customer
  data into DevLogBus.
- Use Browser Tap allow and deny patterns before attaching to broad web
  workflows.
- Block noisy third-party browser sources only after confirming they are not
  relevant.
- Use clear for local screen cleanup and expunge only when daemon replay records
  should be deleted.
- Treat screenshots and demo GIFs as public artifacts; do not capture private
  hostnames, customer data, tokens, or real production URLs.

## Data Retention

The daemon stores records in memory only. The replay buffer is cleared when the
daemon exits. Browser Tap stores configuration in Chrome local extension storage
but does not store captured records there.

Viewers hold their own local UI state while open.

## CORS

The daemon allows cross-origin HTTP requests to support local browser workflows
and Browser Tap. This is another reason the HTTP listener should stay on
loopback or a trusted development network.
