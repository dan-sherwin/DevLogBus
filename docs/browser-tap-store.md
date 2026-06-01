# Browser Tap Chrome Web Store Prep

This is the review-ready Chrome Web Store source material for DevLogBus Browser
Tap.

## Package

Build the upload zip with:

```bash
dev/browser-tap-store-package.sh dist/browser-tap-store
```

The release build also creates the same Web Store-ready zip:

```bash
VERSION=v1.3.1 ./dev/release-artifacts.sh dist/release
```

The Browser Tap zip is intentionally different from the binary release zips:
`manifest.json` is at the zip root so the Chrome Developer Dashboard can upload
it directly.

## Listing

Name:

```text
DevLogBus Browser Tap
```

Short description:

```text
Sends Chrome console, runtime, log, and network events into your local DevLogBus stream.
```

Category:

```text
Developer Tools
```

Detailed description:

```text
DevLogBus Browser Tap is a developer tool for sending browser-side debugging
events into a locally running DevLogBus stream.

Attach it to the active tab when you want Chrome console calls, runtime
exceptions, browser log entries, and network request or response events to
appear beside backend service logs, CLI/TUI records, Linux `journald`, direct
HTTP records, and SDK-published records in DevLogBus. Detach it when you are
done.

The extension is local-first. By default it publishes to
http://127.0.0.1:7423, the HTTP listener exposed by devlogbusd on your machine.
It does not send records to a DevLogBus-owned cloud service.

Capture controls let you turn console, runtime, browser log, or network capture
on and off. Scope controls let you allow or deny specific hosts or URL patterns.
Redaction controls remove common authorization headers, sensitive URL
parameters, and cookie values before records are published.
```

Single purpose:

```text
Publish user-triggered active-tab debugging events to the user's local
DevLogBus development log stream.
```

## Permissions Explanation

- `debugger`: attaches to the active tab after the user clicks Attach so the
  extension can receive console, runtime, browser log, and network events.
- `activeTab`: scopes the attach workflow to the tab the user explicitly acts
  on.
- `tabs`: reads the active tab title and URL so DevLogBus can group browser
  records by source and show useful tab labels.
- `storage`: saves local options such as daemon endpoint, source override,
  capture toggles, allow and deny patterns, and redaction toggles.
- `host_permissions`: limited to `http://127.0.0.1/*` and
  `http://localhost/*` so the extension can post records to the local
  `devlogbusd` HTTP listener without requesting broad site host access.

## Privacy Statement

Privacy policy URL:

```text
https://dan-sherwin.github.io/DevLogBus/browser-tap-privacy.html
```

DevLogBus Browser Tap handles browser debugging data only when the user clicks
Attach for the active tab. Depending on enabled capture toggles, records may
include tab title, tab URL, console messages, runtime exceptions, browser log
messages, request URLs, response status codes, MIME types, and request failure
messages.

The extension stores configuration in Chrome local extension storage. It does
not persist captured debugging records in extension storage.

Captured records are posted to the configured DevLogBus daemon endpoint. The
default endpoint is `http://127.0.0.1:7423` on the user's own machine. DevLogBus
does not operate a cloud collection service for this extension, does not sell
data, does not use the data for advertising, and does not transfer the data to
third parties.

Redaction for authorization tokens, sensitive URL parameters, and cookies is
enabled by default. Users can further restrict capture with allow and deny
patterns before attaching to a tab.

Use of user data is limited to providing and improving the extension's single
purpose: publishing selected browser debugging events to the user's local
DevLogBus development log stream.

## Privacy Dashboard Answers

Suggested user data declarations:

- Web browsing activity: yes. The extension captures active-tab URLs and
  network activity only after the user clicks Attach.
- Website content: yes. Console text, runtime exception text, and browser log
  text may include page-provided content.
- Authentication information: not intentionally collected. Common authorization
  headers, bearer/basic credentials, API keys, sensitive query parameters, and
  cookie values are redacted by default.
- Personal communications, financial information, health information, location,
  or personally identifiable information: not intentionally collected.

Suggested data use:

- Extension functionality.
- Not analytics.
- Not advertising.
- Not personalization.
- Not sold or transferred.

## Screenshots

Chrome's current listing guidance asks for at least one screenshot and prefers
up to five. Capture `1280x800` PNG screenshots with square corners and no
padding.

Store icon:

- `docs/assets/chrome-web-store/store-icon-128.png`: `128x128` PNG matching
  the extension's packaged toolbar icon.

Recommended shot list:

1. `docs/assets/chrome-web-store/browser-tap-detached.png`: Browser Tap popup
   detached, showing endpoint, capture toggles, scope, and redaction controls.
2. `docs/assets/chrome-web-store/browser-tap-attached.png`: Browser Tap popup
   attached to a local development tab.
3. `docs/assets/chrome-web-store/devlogbus-ui-grouped.png`: DevLogBus browser
   UI showing grouped Chrome records beside service logs.
4. `docs/assets/chrome-web-store/devlogbus-ui-details.png`: DevLogBus browser
   UI showing a network failure record selected with details visible.

Small promotional tile:

- `docs/assets/chrome-web-store/small-promo-tile.png`: `440x280` PNG for the
  optional Chrome Web Store small promo tile.

Do not include real secrets, customer hostnames, private URLs, or production
data in screenshots.

## Reviewer Notes

Provide these notes in the Chrome Developer Dashboard test instructions:

```text
This extension is a local developer tool. To test it:

1. Start devlogbusd locally with its default HTTP listener:
   devlogbusd run
2. Open any normal web page, click the DevLogBus Browser Tap toolbar icon, and
   click Attach.
3. Trigger a console message or network request in the page.
4. Open http://127.0.0.1:7423/ and confirm the browser records appear in the
   DevLogBus UI.

The extension only publishes after the user clicks Attach. It posts records to
the local daemon endpoint, defaulting to http://127.0.0.1:7423.
```

## Policy Notes Checked

This prep was written against the Chrome Web Store documentation current on
2026-05-31:

- [Program Policies](https://developer.chrome.com/docs/webstore/program-policies/policies)
  require accurate listing metadata, screenshots, privacy fields, and a privacy
  policy when user data is handled.
- [Declare permissions](https://developer.chrome.com/docs/extensions/develop/concepts/declare-permissions)
  says host permissions are match patterns and should be limited to the feature
  being implemented.
- [Match patterns](https://developer.chrome.com/docs/extensions/develop/concepts/match-patterns)
  documents `http://127.0.0.1/*` and `http://localhost/*` as local daemon host
  patterns.
- [Supplying Images](https://developer.chrome.com/docs/webstore/images)
  documents screenshot sizes of `1280x800` or `640x400`.
- [Publish in the Chrome Web Store](https://developer.chrome.com/docs/webstore/publish)
  documents uploading a valid extension zip in the Developer Dashboard.
