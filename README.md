# myUplink.com exporter

[![license](https://img.shields.io/github/license/webdevops/myuplink-exporter.svg)](https://github.com/webdevops/myuplink-exporter/blob/master/LICENSE)
[![DockerHub](https://img.shields.io/badge/DockerHub-webdevops%2Fmyuplink--exporter-blue)](https://hub.docker.com/r/webdevops/myuplink-exporter/)
[![Quay.io](https://img.shields.io/badge/Quay.io-webdevops%2Fmyuplink--exporter-blue)](https://quay.io/repository/webdevops/myuplink-exporter)

A Prometheus exporter for myUplink heat pumps and connected devices, with
structured state-change logs, Grafana dashboards, and Home Assistant OS apps.

This fork adds background collection and a packaged telemetry pipeline for
Grafana Cloud. The container badges above point to **upstream** images; they do
not contain this fork's changes. Use the local builds described below.

- [Home Assistant OS installation and packaging](#home-assistant-os)
- [Optional local Prometheus retention](#local-metrics-retention-on-home-assistant-os)
- [English and Swedish dashboards](#grafana-dashboards)
- [Grafana alert rules and testing](alerting/README.md)
- [Standalone Raspberry Pi deployment](docs/raspberry-pi.md)

## Background collection

The default is now background polling: `/probe` serves a snapshot and makes no
API calls. Poll delay is `--myuplink.poll-interval=60s`, timeout is
`--myuplink.poll-timeout=55s`, and request spacing is
`--myuplink.request-interval=3s`. Set poll interval to `0` for legacy scrape
collection. Legacy cache settings below apply only to that mode.

Non-null `strVal` changes (including empty strings) and active-alarm transitions
are structured stderr log events. State is in memory and restart emits initial
observations again. Events are generated only by background polling. Snapshot
last success is exposed as `myuplink_poll_last_success_timestamp_seconds`.
429 responses delay all later requests using Retry-After with a 60-second fallback.
Initial auth failure in background mode is recoverable on later polls.

See [the testing guide](docs/testing.md) for build, live verification and
Prometheus setup, and [coverage](docs/api-coverage.md) for remaining collectors.

## Fork improvements

This fork builds on [webdevops/myuplink-exporter](https://github.com/webdevops/myuplink-exporter).
Existing metric names and labels remain available. New metrics are on `/probe`;
`/metrics` continues to expose the exporter's Go runtime metrics.

| Metric | Meaning |
| --- | --- |
| `myuplink_probe_success` | 1 when all requested collections succeeded, otherwise 0 |
| `myuplink_system_has_alarm{systemID}` | System's active-alarm flag; no extra API calls |
| `myuplink_system_device_connected{systemID,deviceID}` | 1 for Connected, otherwise 0; includes devices excluded from point collection |
| `myuplink_system_device_point_timestamp_seconds{systemID,deviceID,parameterID}` | Source timestamp in Unix seconds; omitted if absent from the API |
| `myuplink_system_active_alarms{systemID}` | Active notification count; requires alarm details |
| `myuplink_system_alarm_active{systemID,deviceID,alarmNumber,severity}` | Active notification count per alarm group; requires alarm details |

Enable alarm details with `--myuplink.collect-alarms` or
`MYUPLINK_COLLECT_ALARMS=true`. This adds one API call per system per page when
the cache expires. Severity uses the API's numeric scale (1 is most severe).
Alarm UUIDs and localized messages are excluded from labels. Resolved alarm
groups disappear on refresh and the total becomes zero. An alarm request failure
omits the alarm count rather than claiming zero alarms.

In legacy collection mode (`MYUPLINK_POLL_INTERVAL=0`), systems, points and
optional alarm details are cached for **60 seconds** by default. Override with `/probe?cache=2m`; the duration must be positive. Discovery
and alarm status can be as old as this cache interval. Source point timestamps
can represent the last value change; use the last-success polling metric for
collection freshness rather than assuming an unchanged point is stale. Concurrent probes can
still issue duplicate API reads on a cache miss; an existing entry retains the TTL
from the request that created it. This is a cache, not a global rate limiter.
Select an interval suitable for your device count and API quota.

Device point failures do not suppress healthy devices. API collection failures
return HTTP 200 with `myuplink_probe_success 0`; invalid arguments return HTTP 400.
**Alert on both Prometheus `up` and `myuplink_probe_success`.** A successful probe
can use cached data; it does not mean every device is online or every point is
fresh. Offline devices retain info and connection metrics even though the default
point filter skips them.

System and active-alarm discovery follow pagination. Token refresh failures
return collection errors and may recover on the next probe. Concurrent requests
share synchronized token renewal. Legacy mode retains upstream's fail-fast
startup authentication; background mode retries on later polls.

### Build and run this fork

```sh
git clone https://github.com/fiddeb/myuplink-exporter.git
cd myuplink-exporter
git switch improve-monitoring
go test -race ./...
go build -o myuplink-exporter .
# Set MYUPLINK_AUTH_CLIENTID and MYUPLINK_AUTH_CLIENTSECRET in the environment.
./myuplink-exporter --myuplink.collect-alarms
```

The container badges above refer to upstream images. Build these changes locally
with `docker build -t myuplink-exporter:local .` before running that image.

Prometheus example:

```yaml
scrape_configs:
  - job_name: myuplink
    scrape_interval: 15s
    scrape_timeout: 5s
    metrics_path: /probe
    static_configs:
      - targets: ["myuplink-exporter:8080"]
```

Useful alert expressions (adapt thresholds to your installation):

```promql
myuplink_probe_success == 0
myuplink_system_has_alarm == 1
myuplink_system_device_connected == 0
time() - myuplink_poll_last_success_timestamp_seconds > 300
```

Points still use v2 and upstream's existing value/unit semantics. Future work:
verify v3 scaling against real devices and introduce optional normalized metrics.
Background polling, rate-limit handling and collection timestamps are implemented. Do not
derive energy counters by summing arbitrary instantaneous point values. The
inherited optional total-parameter accumulator is unchanged.

Usage
-----

```
Usage:
  myuplink-exporter [OPTIONS]

Application Options:
      --log.level=[trace|debug|info|warning|error] Log level (default: info) [$LOG_LEVEL]
      --log.format=[logfmt|json]                   Log format (default: logfmt) [$LOG_FORMAT]
      --log.source=[|short|file|full]              Show source for every log message (useful for debugging and bug reports) [$LOG_SOURCE]
      --log.color=[|auto|yes|no]                   Enable color for logs [$LOG_COLOR]
      --log.time                                   Show log time [$LOG_TIME]
      --myuplink.collect-alarms                    Collect active alarm details [$MYUPLINK_COLLECT_ALARMS]
      --myuplink.url=                              Url to myUplink API (default: https://api.myuplink.com) [$MYUPLINK_URL]
      --myuplink.auth.clientid=                    ClientID from myUplink [$MYUPLINK_AUTH_CLIENTID]
      --myuplink.auth.clientsecret=                ClientSecret from myUplink [$MYUPLINK_AUTH_CLIENTSECRET]
      --myuplink.device.allowed-connectionstates=  Allowed device connection states (default: Connected) [$MYUPLINK_DEVICE_ALLOWED_CONNECTIONSTATES]
      --myuplink.device.calc-total-parameters=     Calculate total metrics for these parameters (eg. energey log parameters) [$MYUPLINK_DEVICE_CALC_TOTAL_PARAMETRS]
      --server.bind=                               Server address (default: :8080) [$SERVER_BIND]
      --server.timeout.read=                       Server read timeout (default: 5s) [$SERVER_TIMEOUT_READ]
      --server.timeout.write=                      Server write timeout (default: 10s) [$SERVER_TIMEOUT_WRITE]

Help Options:
  -h, --help                                       Show this help message
```

## Auth (Client id and secret)

Register an application at [myUplinkDEVS](https://dev.myuplink.com/login) to get
client id and secret for your application.

## HTTP Endpoints

| Endpoint   | Description                             |
|------------|-----------------------------------------|
| `/metrics` | Default prometheus golang metrics       |
| `/probe`   | Probe devices metrics from myuplink.com |

### `/probe`

| GET parameter | Default                   | Required | Multiple | Description                                     |
|---------------|---------------------------|----------|----------|-------------------------------------------------|
| `cache`       |                           | no       | no       | Cache time in time.Duration (eg. `30s` or `5m`) |


## Metrics

| Metric                                 | Description                              |
|----------------------------------------|------------------------------------------|
| `myuplink_system_info`                 | System information                       |
| `myuplink_system_device_info`          | Device information (belongs to a system) |
| `myuplink_system_device_point`         | Device parameter metric point            |

## API errors and coverage

API failures are returned as `*myuplink.APIError`; token endpoint failures use
`*myuplink.OAuthError`. Use `errors.As` to inspect errors even when wrapped.
Actual HTTP status remains authoritative when the body reports a different status,
is HTML/empty/malformed, or exceeds the 64 KiB error-body limit. Error codes accept
string or integer encodings because the published Swagger does not define the
ErrorCodes schema. Unknown JSON fields are ignored. `Retry-After` is honored by the shared request limiter, with a 60-second
fallback on HTTP 429. Failed background polls are retried on a later polling cycle.

Structured probe error logs expose operation, HTTP status, error code and body
parsing status (plus source timestamp for API errors). Details, additional data,
OAuth descriptions/URIs and raw response bodies are retained where applicable
for callers but are not logged automatically. Transport-level HTTP trace/debug
logging is separate from these application logs.

See [API coverage](docs/api-coverage.md) for the reviewed endpoints, missing
collectors and live-device validation still required. This exporter is not yet
comprehensive across all monitoring features in the public API.

## Raspberry Pi and Grafana Cloud

See [the Raspberry Pi deployment guide](docs/raspberry-pi.md) and `deploy/` for
a systemd service, root-only environment templates, Alloy journal collection,
Loki forwarding and Prometheus remote write. Text events include `observed_at`;
formatted numeric strings are now suppressed unless
`--myuplink.log-numeric-strings` is enabled. Empty strings still count as textual
state changes. Numeric enum comparisons preserve fractional values.

## Home Assistant OS

A read-only **myUplink Exporter custom integration** now creates Home Assistant
entities directly from the exporter's cached Prometheus endpoint. Follow the
[integration installation and test guide](homeassistant/INTEGRATION.md).
Set the app option `cloud_enabled: false` to run without Grafana Cloud credentials
or an Alloy process; the default `true` preserves existing Cloud installations.
Prometheus export remains available in both modes.


Use the two local apps under `homeassistant/` on **aarch64** (including Raspberry
Pi 5 with 64-bit HAOS) or **amd64**. They are Supervisor-managed containers, not
Debian packages installed on the HAOS host. Do not run Grafana Cloud's Linux
installer or `dpkg` on HAOS. Keep Protection mode enabled; neither app requires
host networking, a Docker socket or a Home Assistant API token.

| Package | Version | Purpose |
| --- | --- | --- |
| [myUplink Cloud telemetry](homeassistant/myuplink_cloud/DOCS.md) | 0.4.1 | Go exporter + optional Grafana Alloy; local entities and/or Cloud telemetry |
| [Prometheus Local](homeassistant/prometheus_local/DOCS.md) | 0.1.0 | Optional persistent local metrics history |

### How data flows

1. The exporter polls myUplink in the background: 60-second delay after a poll,
   55-second poll timeout and at least 3 seconds between API requests by default.
   It exposes the latest snapshot on `/probe` at port 8080. `/metrics` contains
   the exporter's own Go/runtime metrics, not heat-pump data.
2. Alloy scrapes `/probe` every 15 seconds and forwards metrics using Prometheus
   remote write. More frequent scrapes do not generate additional myUplink calls.
3. The app writes exporter JSON logs to `/data/exporter.jsonl` and its Log tab.
   Alloy reads that file and sends it to Loki. Startup observations, text changes,
   alarm transitions and collection errors are available as structured events.
   Unchanged text values are not repeatedly logged.
4. Optional Prometheus Local independently scrapes the same snapshot over the
   internal app network. It stores metrics locally; it does not store Loki logs.

Alloy keeps its state under `/data/alloy`. The exporter log rotates at 5 MiB
with two backups. Only bounded `level` and `event` fields are added as log labels;
parameter IDs and values stay in JSON. Source timestamps stay in the event body.
The Python entrypoint supervises both processes and stops the app if either exits.

### Install the telemetry app

This repository currently provides **local app build directories**, not a
one-click Supervisor repository or prebuilt fork images. In Advanced SSH & Web
Terminal, with the local app folder mounted at `/addons`:

```sh
test -d /addons || { echo '/addons is unavailable; check the SSH app mount or use Samba'; exit 1; }
git clone --branch improve-monitoring --single-branch https://github.com/fiddeb/myuplink-exporter.git /tmp/myuplink-exporter-src
test ! -e /addons/myuplink_cloud && cp -R /tmp/myuplink-exporter-src/homeassistant/myuplink_cloud /addons/myuplink_cloud
```

Use a fresh temporary path if it already exists. If the app directory exists,
follow the upgrade procedure instead of overwriting it during initial setup.

1. Open **Settings → Apps → Install app** (older versions: **Add-ons → Add-on
   store**), reload/check for updates, and find **myUplink Cloud telemetry** under
   **Local apps**.
2. Select **Install**. Supervisor builds the container for your architecture;
   the first build needs internet access and can take several minutes.
3. In **Configuration**, enter the myUplink client ID/secret. Keep
   `cloud_enabled: true` and fill Cloud endpoints, usernames and tokens for
   Grafana Cloud, or set it to `false` for exporter-only operation. Save, Start,
   and enable Start on boot.
4. Check the app Log tab and the queries in the [verification guide](homeassistant/myuplink_cloud/DOCS.md#verify).
   The first poll may temporarily return HTTP 503; then `up` and
   `myuplink_probe_success` should be 1.

| App options | Where they come from |
| --- | --- |
| `cloud_enabled` | Whether to start Alloy; default true for existing installations |
| `client_id`, `client_secret` | Your myUplink developer application |
| `metrics_url`, `metrics_user`, `metrics_token` | Cloud Metrics remote-write endpoint, Metrics ID and token with `metrics:write` |
| `loki_url`, `loki_user`, `loki_token` | Cloud Logs push endpoint, Logs ID and token with `logs:write` |
| `instance` | Stable label, for example `homeassistant` |
| `poll_interval`, `poll_timeout`, `request_interval` | Polling controls in seconds; defaults 60, 55, 3 |

Use the exact endpoints from your Grafana Cloud stack; the Metrics ID and Logs ID
are different fields. Both destinations are required only when `cloud_enabled`
is true. With it false, Cloud fields may be empty or omitted.
Store credentials in the app's Configuration UI, not in source files. Supervisor
provides `/data/options.json`; the entrypoint passes the required credentials to
each process through its environment. Treat app backups as sensitive.

### Packaging and updates

Each app directory is a self-contained Docker build context:

- `config.yaml`: app metadata, version, architectures, option schema and ports.
- `Dockerfile`: upstream image versions and build stages.
- `run.py`: validated options and process startup; the telemetry app also ships
  `config.alloy` for metrics/log forwarding.
- `DOCS.md`: installation, configuration, troubleshooting and verification.

The telemetry Dockerfile builds the Go exporter from pinned commit
`ab61837c50d34e339fe47df7eb9f019803044a50`, copies Alloy **v1.21.3** from its
upstream image, and optionally runs Alloy alongside the exporter in a Python-based container. **Copying the latest
app directory does not automatically build the latest exporter branch**: exporter
code changes require updating this commit pin and bumping the app version.
Prometheus Local copies Prometheus/promtool **v3.15.1** from its upstream image.
Image tags are version-pinned, not digest-pinned.

To upgrade, review the branch changes, copy the updated app directory contents
into its existing `/addons/` directory, reload the app store, then update/rebuild
and restart the app. Do not uninstall it to upgrade. A source-file copy alone
does not update the running container. Saved options and `/data` persist across
normal updates; uninstalling may remove app data. Back up before upgrading.
Dashboard JSON is imported separately into Grafana; app updates do not update
dashboards or alerts automatically.

Version 0.3.0 adds parsed `level`/`event` Loki labels; old logs remain readable
because dashboard queries also parse the JSON body. The pipeline has been used
with live Grafana Cloud data, but each new image still needs an actual build and
runtime check on the target HAOS architecture; local tests alone do not validate
Supervisor installation.

### Credentials in local files

For standalone deployment, copy the reviewed `.env.example` templates and fill
in your own values locally. `.env`, `.env.*` and `*.env` are excluded from Git
and the root Docker build context; example templates remain versionable. Ignore
rules do not protect files already tracked by Git and do not replace secrets
scanning. Never commit tokens, app options or backups containing credentials.

## Local metrics retention on Home Assistant OS

Install [Prometheus Local](homeassistant/prometheus_local/DOCS.md) as the second
local app when you want history beyond your Grafana Cloud retention. It scrapes
`local-myuplink-cloud:8080` by default; verify the telemetry app's **Hostname** on
its Info tab. Do not use `localhost`, which refers to Prometheus's own container.
The telemetry app exposes port 8080 internally without publishing it on the host.

Defaults are a 60-second scrape interval and **180 days or 2 GiB of persisted
blocks**, whichever limit is reached first. `/data/tsdb` survives restarts and
normal updates. WAL/head and compaction require extra disk space; the retention
size is not a filesystem quota. Local history starts when collection starts;
it cannot restore already-expired Cloud samples.

Prometheus publishes its unauthenticated UI/query API on host port 9090 by
default. Keep it on a trusted LAN, or disable the host mapping when only other
apps need it. A local Grafana can query `http://local-prometheus-local:9090`
(verify the actual Hostname). Grafana Cloud cannot automatically reach private
HAOS addresses; that needs separate private connectivity. Local retention does
not extend Cloud retention or Loki log retention.

## Grafana dashboards

English and Swedish versions are included in both layouts:

| Layout | English | Swedish |
| --- | --- | --- |
| Dynamic tabs (schema v2) | [English dynamic dashboard](dashboards/ctc-ecozenith-i360.dynamic.en.json) | [Swedish dynamic dashboard](dashboards/ctc-ecozenith-i360.dynamic.json) |
| Classic sections | [English classic dashboard](dashboards/ctc-ecozenith-i360.en.json) | [Swedish classic dashboard](dashboards/ctc-ecozenith-i360.json) |

Dynamic tabs group **Overview**, **Space heating**, **Hot water**, **Heat pump**,
and **Logs & troubleshooting**. The overview includes key temperatures, system
health and a compressor/heating/defrost state timeline. Detailed tabs include
metrics and readable Loki logs. The English version has a separate dashboard UID
so it can coexist with Swedish. Classic and dynamic layouts of the same language
share a UID: importing one replaces the other.

Import the JSON into Grafana and select Prometheus, Loki, Instance, System and
Device. Raw API values and enum text retain their source language; the English
version translates panel labels, descriptions, mappings and log formatting.
See [dashboard instructions and interpretation notes](dashboards/README.md).

## Alerts

[Grafana alert rules](alerting/README.md) cover active system alarms, an offline
device, and missing/stale collection. They are configured separately from the
Home Assistant apps and dashboard. Adapt the data source, instance and contact
point before reuse, and test notification delivery. The export uses Grafana
file-provisioning format, not dashboard JSON or a Prometheus rule file.

### Unavailable CTC measurements

The exporter omits numeric CTC EcoZenith points whose API value is exactly
`-32768`, observed for unavailable return temperature and hot-water capacity.
Their enum, calculated-total and measurement-timestamp samples are omitted too.
Other points continue to be exported; valid readings resume automatically.
Raw string events remain available for diagnosis. No zero or last-known value
is substituted. Previously stored samples remain in Prometheus and Home Assistant
Recorder; select a recent time range when checking the fix.
