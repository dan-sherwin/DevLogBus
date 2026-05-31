# DevLogBus Browser Tap

Chrome Manifest V3 extension that attaches Chrome's debugger transport to the active tab and publishes browser-side debug events to `devlogbusd`.

The extension icons live in `icons/` and are sized for Chrome's toolbar,
extension management page, and install surfaces.

## What It Captures

- console calls from the tab
- runtime exceptions
- browser log entries
- network requests, responses, and failures

Records are posted to the daemon's HTTP API:

```text
POST http://127.0.0.1:7423/api/records
```

## Load Locally

1. Start `devlogbusd` with the HTTP listener enabled.
2. Open `chrome://extensions`.
3. Enable Developer mode.
4. Choose Load unpacked.
5. Select this directory.
6. Open a page, click the DevLogBus extension, and Attach.

The default endpoint is `http://127.0.0.1:7423`. Use the popup to override the
emitted source name or, in unpacked development builds with matching host
permissions, point at another trusted daemon HTTP address.

The Chrome Web Store package grants host access only for localhost daemon
endpoints. If you need a remote Browser Tap target during private development,
load the extension unpacked with a manifest that grants that specific trusted
daemon host.

## Store Package

Create the Chrome Web Store upload zip with:

```bash
dev/browser-tap-store-package.sh dist/browser-tap-store
```

The generated zip has `manifest.json` at the archive root and excludes macOS
metadata files.

## Scope And Redaction

The popup can limit capture with allow and deny patterns. Add one host or URL
pattern per line. Blank allow patterns mean any page is allowed; deny patterns
always win.

Examples:

```text
localhost:5173
*.example.com
https://api.example.com/*
```

The browser tap also redacts common secrets before publishing records. The
popup controls authorization-token redaction, sensitive URL parameter redaction,
and cookie redaction. These controls affect messages, attributes, tab URLs,
stack-frame URLs, and network URLs before records are sent to DevLogBus.

## Source Names

Without an override, records use:

```text
chrome:<host>
```

For example:

```text
chrome:localhost:5173
```

Use the popup source override when a shorter app name is more useful, such as `chrome:tenant-ui`.

Network events keep the request target as the record source, such as
`chrome:scpapi.review.spacelink.com`, while also adding `sourceGroup` for the
owning browser tab. The DevLogBus UI uses that parent group to show browser tab
traffic as a nested source when one tab talks to multiple targets.

In DevLogBus, the parent tab source can be viewed as one merged timeline or as
child sources split by console/runtime records and network targets. The parent
group or an individual child source can also be popped into a separate window
and reattached later.

When the tab title is available, the UI displays the owning Chrome source with
the title and host, such as `chrome:Spacelink Cloud Portal (localhost:3010)`.
The underlying record source stays unchanged for filtering and expunging.

Any debugger detach publishes a `WARN` record with a loud
`*** DEVLOGBUS BROWSER TAP DETACHED ***` message. That includes explicit popup
detaches, Chrome's debugger banner being dismissed, DevTools taking over the
debugger session, and tab close events.
