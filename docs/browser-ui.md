# Browser UI

The embedded browser UI is served by `devlogbusd` at `http://127.0.0.1:7423/`
by default. It is the main visual debugger for coalescing full-stack development
logs into side-by-side source panes or one merged timeline.

## Connection

The UI reads initial records through the HTTP replay API and then follows the
live stream through `/api/stream`. The connection indicator reports whether the
stream is connected or reconnecting.

The About button shows daemon build information, broker endpoint, HTTP address,
optional TCP address, record retention, and echo state.

## Login Protection

DevLogBus is open by default. A fresh daemon install has no users and does not
require a login.

The Settings button opens user management for the embedded UI. Add users with:

- username
- display name
- password

After at least one user exists, login mode can be toggled on or off from the
same Settings dialog. When login mode is on, the browser UI requires a session
cookie and protects the log/about/record HTTP APIs that expose buffered records.
When login mode is off, the UI and APIs are open again.

## Merged View

Merged view shows one chronological timeline across all visible sources. This
is the best starting point when you want to see backend service records, Browser
Tap records, CLI records, and journal records in exact order.

Use merged view when:

- You are trying to understand a request lifecycle.
- You care about order more than ownership.
- There are only a few active sources.

## By-Source View

By-source view splits records into panes. Each pane has its own level filters,
pause, bottom-follow, details, clear, expunge, and popout controls.

Use by-source view when:

- One source is noisy.
- You need to compare backend service logs beside browser network records.
- Browser Tap has created several child sources for one tab.

## Browser Source Groups

Browser Tap publishes records with a `sourceGroup` attribute. DevLogBus treats
that value as a parent group. The child sources remain real sources, such as:

```text
chrome:localhost:5173
chrome:api.localhost:8080
chrome:auth.localhost:8443
```

The parent group can be viewed as:

- One merged browser-tab timeline.
- Child panes split by console/runtime records and network targets.

The displayed label can include the tab title and host when Browser Tap provides
the tab metadata.

## Hide And Block

Hide removes a source or group from the current view. It is a viewer-level
choice and does not stop the daemon from receiving records.

Block suppresses future records from that source in the browser UI until the
source is unblocked. Use it for persistent noise such as analytics, hot reload
chatter, or known irrelevant third-party calls.

The blocked-source list can restore blocked sources later.

## Popouts

Source groups and individual sources can be popped into separate browser
windows. A popout receives the same stream but is scoped to that group or
source. Popouts can be reattached later.

Popouts are useful when:

- A browser tab group deserves its own monitor.
- You want backend service records in the main window and browser/network
  records elsewhere.
- A source is important enough to keep visible while the main UI changes modes.

## Clear And Expunge

Clear removes visible records from the current UI state only. It does not delete
daemon replay records.

Expunge deletes matching records from the daemon replay buffer. Use it when you
want refreshes and new subscribers to stop replaying old records.

## Details

Record details show canonical fields and structured attributes:

- `time`
- `level`
- `source`
- `sourceGroup`
- `message`
- `attrs`

Enable inline details on a pane when attributes are more useful than compact log
rows.

## Help

The Help button explains the intended workflow: start merged, split by source
when noise appears, use per-pane controls, hide or block carefully, clear only
the viewer, and expunge only the daemon replay buffer.
