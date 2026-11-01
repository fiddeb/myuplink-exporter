# Test the exporter

## 1. Get and validate the code

Use Go 1.27.3 or the toolchain requested in go.mod. No credentials are needed for
automated tests; they run against local httptest servers.

```sh
git clone https://github.com/fiddeb/myuplink-exporter.git
cd myuplink-exporter
git switch improve-monitoring
git pull --ff-only
go test -race ./...
go vet ./...
go build -o myuplink-exporter .
```

Expected: all tests pass. They cover API/OAuth error parsing, bounded error bodies,
pagination, token refresh and bounded 401 recovery, explicit JSON decoding,
concurrent calculated totals, partial collection failures, offline devices, string
change deduplication, empty-string transitions, alarm transitions, snapshot serving,
Retry-After and cancellation while rate-limited, queued or waiting for token renewal.
These tests do not verify the
manufacturer's numeric scaling or account-specific endpoint access.

## 2. Start against your own installation

Create a client in the myUplink developer portal associated with the account
that owns the installation. Only READSYSTEM is requested. Enter credentials in
your shell without putting them in command history (Bash example):

```sh
read -r -p 'Client ID: ' MYUPLINK_AUTH_CLIENTID
read -r -s -p 'Client secret: ' MYUPLINK_AUTH_CLIENTSECRET
printf '\n'
export MYUPLINK_AUTH_CLIENTID MYUPLINK_AUTH_CLIENTSECRET
./myuplink-exporter \
  --server.bind=127.0.0.1:8080 \
  --log.format=json --log.time \
  --myuplink.collect-alarms \
  --myuplink.poll-interval=60s \
  --myuplink.poll-timeout=55s \
  --myuplink.request-interval=3s \
  2>myuplink.log
```

This only reads installation data. Logs may include string values and device IDs;
keep them private. All structured logs currently use stderr, including change
events. Route this file or the container's stderr to Alloy/Loki if desired.
Do not enable HTTP wire tracing.

The first request to /probe may return 503 until the initial poll finishes.
Polling starts immediately; subsequent polls start 60 seconds after the preceding
poll finishes. Each API call is spaced by at least 3 seconds. The default
55-second deadline may need increasing for installations with many devices or
alarm pages. This spacing is configurable, not a claim about your account's quota.
Initial authentication failures now appear as failed polls and can recover without
restarting. /healthz and /readyz retain the upstream process-only checks; use
/probe and its success/last-success metrics for collection readiness.

## 3. Inspect metrics and logs

In another terminal:

```sh
curl -fsS http://127.0.0.1:8080/probe > /tmp/myuplink-metrics.txt
rg 'myuplink_(probe_success|poll_last_success|system_has_alarm|system_device_connected|system_active_alarms|system_device_point_timestamp)' /tmp/myuplink-metrics.txt
rg 'point_observed|point_changed|alarm_observed|alarm_active|alarm_no_longer_active' myuplink.log
```

Expected:

- `myuplink_probe_success 1` after a successful poll.
- `myuplink_poll_last_success_timestamp_seconds` advances on successful polls.
- Devices visible in myUplink have appropriate connection status.
- No current alarms means active count zero, not a fabricated alarm event.
- First textual strVal values generate point_observed; formatted numeric strings are suppressed by default (enable --myuplink.log-numeric-strings to include them). Unchanged values do not
  produce duplicate events. Empty strings are supported. No string points means
  no point events, which can be normal for a device.
- Existing alarms generate alarm_observed on first successful alarm collection.
  A later absence produces alarm_no_longer_active. It does not prove a specific
  reset reason. Failed alarm collection must not fabricate resolved alarms.

Scrape /probe repeatedly: it must only return the stored snapshot, without
additional API collection or repeated events. `cache` query parameters have no
effect in background mode. Change detection is in memory: restart produces new
observations. Polling can miss transient changes between polls. Alarm logs
currently identify alarm IDs; localized alarm descriptions are not emitted.

Do not intentionally trigger equipment faults. Compare an existing alarm or a
naturally occurring transition against the app. If a normal, safe change is made
in the manufacturer's app, check for its corresponding string event on the next
poll only if the API exposes that state as strVal.

## 4. Validate values against the manufacturer app

Compare at least outdoor temperature, supply/return temperature, compressor state
and available energy totals. Record parameter ID, metric value, unit, app value
and observation time. Allow for cloud update delay. The exporter still uses v2;
do not assume scaleValue or energy units are normalized. Do not switch dashboards
to rate() merely because a metric name ends in _total. V3 and normalized units
require real-device verification before implementation.

## 5. Connect Prometheus

```yaml
scrape_configs:
  - job_name: myuplink
    scrape_interval: 15s
    scrape_timeout: 5s
    metrics_path: /probe
    static_configs:
      - targets: ["127.0.0.1:8080"]
```

The target address is from Prometheus's network namespace. For containers or
remote Prometheus, choose a reachable bind/target address and restrict access.
A fast scrape does not increase API polling frequency.

Check these expressions separately:

```promql
up{job="myuplink"} == 0
myuplink_probe_success{job="myuplink"} == 0
time() - myuplink_poll_last_success_timestamp_seconds{job="myuplink"} > 300
myuplink_system_device_connected{job="myuplink"} == 0
myuplink_system_has_alarm{job="myuplink"} == 1
time() - myuplink_system_device_point_timestamp_seconds{job="myuplink"} > 600
```

Adjust thresholds to the installation's normal update frequency.

## 6. Verify failure handling without stressing the API

Rely on automated fixtures for 429, malformed responses and oversized bodies.
Do not deliberately exceed the real service's rate limit. A real 429 applies a
shared cooldown using Retry-After (seconds or HTTP date); absent/invalid values
use 60 seconds. Resty's existing bounded retry behavior remains in place; each
attempt passes through the same gate and poll deadline.

On failure, expect probe_success=0 and structured API/OAuth logs. The last success
time must stop advancing. Healthy devices from that poll remain visible. Source
string payloads, API details/data and raw OAuth descriptions are not metric labels.
If stopped polling or missing data is observed, preserve sanitized metrics/logs
for investigation; do not share client secrets or unreviewed raw API bodies.

Remaining collectors are listed in api-coverage.md. This release does not yet add
v3, aid mode, smart home mode, zones, pending firmware or crash-log ingestion.
