# Compatibility Policy

This policy describes the intended v1 compatibility surface for public users.
Until a `v1.0.0` release is tagged, DevLogBus may still make breaking changes
when needed to get the public API right.

## Versioning

After `v1.0.0`, DevLogBus should use semantic versioning:

- Patch releases fix bugs and documentation.
- Minor releases add compatible features.
- Major releases may make breaking changes.

## Record Schema

The v1 record fields are:

```text
id
time
level
source
message
attrs
```

Compatibility promises:

- Existing JSON field names remain stable through v1.x.
- New optional fields may be added.
- Consumers should ignore unknown fields.
- `attrs` remains a JSON object for structured metadata.
- `sourceGroup` in `attrs` remains the public grouping convention.

## Levels

The canonical public levels are:

```text
DEBUG
INFO
WARN
ERROR
```

Aliases such as `warn`, `warning`, `err`, and `dbg` may continue to normalize
to canonical levels.

## HTTP API

The following endpoints are part of the v1 compatibility surface:

```text
GET    /api/health
GET    /api/about
GET    /api/records
POST   /api/records
DELETE /api/records/expunge
GET    /api/stream
```

Compatibility promises:

- Existing paths and methods remain stable through v1.x.
- Existing query parameters remain supported.
- Response objects may gain optional fields.
- SSE `record` events remain JSON encoded DevLogBus records.

## Socket Wire Protocol

The newline-delimited JSON envelope protocol is public for SDK and advanced CLI
use:

```text
log
subscribe
replay_complete
expunge
expunge_result
error
```

Envelope field names should remain stable through v1.x.

## Go SDK

Packages intended for public use:

```text
github.com/dan-sherwin/devlogbus/pkg/protocol
github.com/dan-sherwin/devlogbus/pkg/client
github.com/dan-sherwin/devlogbus/pkg/sloghandler
github.com/dan-sherwin/devlogbus/pkg/runtime
```

Compatibility promises:

- Public exported types and functions should not be removed during v1.x without
  a deprecation period.
- New fields may be added to option structs.
- Existing behavior for default endpoints should remain platform-stable.
- Queueing and reconnect behavior may be tuned, but application logging should
  remain non-blocking by default.

## CLI

The main public commands are:

```text
devlogbus emit
devlogbus tail
devlogbus tui
devlogbus expunge
devlogbus endpoint
devlogbus version
devlogbus buildinfo
```

Compatibility promises:

- Existing command names should remain through v1.x.
- Existing common flags should remain through v1.x.
- Output intended for humans may evolve.
- Automation should prefer JSON/HTTP/socket APIs where stable parsing matters.

## Browser Tap

Browser Tap compatibility centers on record shape and source grouping:

- It publishes DevLogBus record JSON to `/api/records`.
- It uses `chrome:<host>` source names by default.
- It uses `attrs.sourceGroup` to group child sources under the owning tab.
- It keeps redaction controls enabled by default.

Chrome permission names and review requirements may change outside DevLogBus
control.

## Deprecation

When a public v1 feature needs to change, prefer:

1. Add the replacement.
2. Document the old behavior as deprecated.
3. Keep the deprecated behavior through at least one minor release.
4. Remove it only in the next major release unless there is a security reason.
