# Public API coverage

Reviewed 2026-10-31 against the [published OpenAPI definition](https://api.myuplink.com/swagger/docs/public-v2/swagger.json)
and [Swagger UI](https://api.myuplink.com/swagger/index.html). This is a static
coverage review, not proof of availability on a specific account or device.

## Current collection

| API area | Exporter coverage | Remaining work |
| --- | --- | --- |
| `GET /v2/systems/me` | Paginated system/device discovery, system info, alarm flag, device connection state and current firmware from discovery | Confirm completeness against a live account |
| `GET /v2/devices/{deviceId}/points` | Non-null numeric values, enums, source timestamps | String change events are emitted during background polling; validate units and scaling |
| `GET /v3/devices/{deviceId}/points` | Not collected | Verify scaled values against v2/device displays before opting in; avoid silently changing existing values |
| `GET /v2/systems/{systemId}/notifications/active` | Optional paginated active counts and grouped alarm counts | Lifecycle logs by alarm ID implemented; localized details and durable deduplication remain |
| `POST /oauth/token` | Client credentials, synchronized renewal, typed OAuth errors | Rate-aware retry policy and handling an unexpectedly rejected access token |
| Failed API responses | Typed APIError, HTTP status authoritative, 64 KiB error-body parsing limit, Retry-After retained | Shared request spacing and Retry-After cooldown implemented; request counters remain |

## Additional monitoring candidates

| Endpoint/area | Priority and intended output |
| --- | --- |
| Device `firmware-info` | Medium: client method exists but is not called by probe; export pending/desired firmware information with a slow refresh |
| Device `aidMode` | High: enum metric for an operational state, provided it is available and not already represented by points |
| System `smart-home-mode` | High: enum metric, if available and not duplicated by points |
| Device `smart-home-zones` and `smart-home-categories` | Medium: inspect zone IDs, properties and values before defining metrics; avoid duplicate point series |
| Device `parameter-metadata`, `product-metadata`, system categories | High for semantic validation: use for discovery/units/enum interpretation where needed; not all metadata should become metric labels |
| Notifications (all) and notification details | Medium: event history can help log ingestion/recovery; do not repeatedly emit all historic alarms |
| Device `crash-logs` | Optional diagnostics: the current OpenAPI response has no structured success schema; inspect a real response before assuming JSON log records or implementing ingestion |
| System subscriptions | Optional capability discovery; do not assume endpoint presence means account/device access |
| Device details and system details | Fetch only if discovery lacks required information |
| Latest individual point | Usually redundant with batch points; use only for a demonstrated missing parameter |
| Historical and aggregated point data | Separate backfill/analysis workflow; do not expose historic records as current scrape values |
| Ping/protected-ping | Optional API diagnostics; existing collection success is usually more useful and avoids extra calls |

## Outside normal exporter collection

Firmware downloads/uploads, device/system setting changes, alarm resets, spot-price
submission, product registrations and browser login pages do not provide normal
read-only monitoring data. Internal CRM/SSG/AitHp24 endpoints are not assumed to
be supported public integrations merely because Swagger lists them. The exporter
must not trigger device changes to collect telemetry.

## Completion criteria

A useful monitoring exporter needs more than endpoint enumeration:

1. Verify numeric values, scaleValue, enum mappings and units using actual device fixtures.
2. Persist change-event state if restart deduplication is required; current state is in memory.
3. Background polling is separate from scrapes and respects Retry-After; verify pacing and quotas on the target account.
4. Expose request outcomes and last-success times without free-text or identifier-heavy labels.
5. Make optional collectors/capabilities explicit: distinguish unavailable, forbidden, failed and empty results.
6. Test a real installation before claiming comprehensive coverage.
