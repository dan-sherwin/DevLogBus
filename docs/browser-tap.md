# Browser Tap

Browser Tap is the Chrome extension that publishes browser-side debugging
events into DevLogBus.

It is intended for local development. It does not publish until the user clicks
Attach for the active tab.

## Install Locally

Until the Chrome Web Store listing is public:

1. Start `devlogbusd`.
2. Open `chrome://extensions`.
3. Enable Developer mode.
4. Click Load unpacked.
5. Select `extensions/chrome-devlogbus`.
6. Open a normal web page.
7. Click the DevLogBus Browser Tap toolbar icon.
8. Click Attach.

The default daemon HTTP endpoint is:

```text
http://127.0.0.1:7423
```

## Permissions

Browser Tap requests:

- `debugger`: attaches to the active tab after the user clicks Attach so Chrome
  can deliver console, runtime, browser log, and network events.
- `activeTab`: scopes the attach flow to the tab the user acts on.
- `tabs`: reads the active tab title and URL for source grouping and labels.
- `storage`: saves endpoint, source override, capture toggles, filters, and
  redaction settings.
- `host_permissions`: local daemon HTTP endpoints.

The Chrome Web Store package grants host access only to:

```text
http://127.0.0.1/*
http://localhost/*
```

Private unpacked builds can add a specific trusted remote daemon host if needed
for development.

## Capture Surface

Browser Tap can capture:

- console calls
- runtime exceptions
- browser log entries
- network requests
- network responses
- network failures

Each capture category has a popup toggle.

## Scope Filters

Allow and deny patterns limit what Browser Tap publishes. Deny patterns always
win. Empty allow patterns mean any page is allowed.

Examples:

```text
localhost:5173
*.example.com
https://api.example.com/*
```

Use filters before attaching if the tab may load noisy or sensitive third-party
URLs.

## Redaction

Redaction is enabled by default for:

- authorization tokens
- sensitive URL parameters
- cookies

Redaction applies to messages, attributes, tab URLs, stack-frame URLs, and
network URLs before records are sent to DevLogBus.

## Sources And Groups

Without an override, Browser Tap source names use:

```text
chrome:<host>
```

Network events use the request target host as the record source while also
setting `attrs.sourceGroup` for the owning browser tab.

Example:

```json
{
  "source": "chrome:api.localhost:8080",
  "message": "GET /api/orders -> 500",
  "attrs": {
    "sourceGroup": "chrome:localhost:5173",
    "tabTitle": "Local Checkout"
  }
}
```

The UI and TUI can show the parent tab as one merged timeline or split it into
child sources.

## Detach Warnings

Any debugger detach publishes a `WARN` record:

```text
*** DEVLOGBUS BROWSER TAP DETACHED ***
```

This includes popup detaches, tab close events, Chrome debugger detach events,
and DevTools taking over the debugger session.

## Chrome Web Store Package

Create an upload zip with:

```bash
dev/browser-tap-store-package.sh dist/browser-tap-store
```

See [Browser Tap Chrome Web Store Prep](browser-tap-store.md).
