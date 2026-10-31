# myUplink.com exporter

[![license](https://img.shields.io/github/license/webdevops/myuplink-exporter.svg)](https://github.com/webdevops/myuplink-exporter/blob/master/LICENSE)
[![DockerHub](https://img.shields.io/badge/DockerHub-webdevops%2Fmyuplink--exporter-blue)](https://hub.docker.com/r/webdevops/myuplink-exporter/)
[![Quay.io](https://img.shields.io/badge/Quay.io-webdevops%2Fmyuplink--exporter-blue)](https://quay.io/repository/webdevops/myuplink-exporter)

A Prometheus exporter for myuplink.com device metrics

## Background collection (current branch)

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

Systems, points and optional alarm details are cached for **60 seconds** by
default. Override with `/probe?cache=2m`; the duration must be positive. Discovery
and alarm status can be as old as this cache interval. Source point timestamps
allow detection of data that stopped updating upstream. Concurrent probes can
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
share synchronized token renewal. Initial startup authentication retains
upstream's fail-fast behavior.

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
    scrape_interval: 60s
    scrape_timeout: 25s
    metrics_path: /probe
    params:
      cache: ["60s"]
    static_configs:
      - targets: ["myuplink-exporter:8080"]
```

Useful alert expressions (adapt thresholds to your installation):

```promql
myuplink_probe_success == 0
myuplink_system_has_alarm == 1
myuplink_system_device_connected == 0
time() - myuplink_system_device_point_timestamp_seconds > 600
```

Points still use v2 and upstream's existing value/unit semantics. Future work:
verify v3 scaling against real devices, add rate-limit-aware background polling
and collection timestamps, and introduce optional normalized metrics. Do not
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
ErrorCodes schema. Unknown JSON fields are ignored. `Retry-After` is retained for
future scheduling; this change does not implement a new retry policy.

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

For aarch64 Home Assistant OS (including Raspberry Pi 5), use the combined local
app in [homeassistant/myuplink_cloud](homeassistant/myuplink_cloud/DOCS.md).
It bundles the exporter and Alloy with private file-based log transport, so no
host systemd installation or Docker socket access is needed. Follow its local
installation and Grafana Cloud configuration guide. The first full container
build and Supervisor installation still need validation on a real HAOS host.

## Local metrics retention on Home Assistant OS

[Prometheus Local](homeassistant/prometheus_local/DOCS.md) is a separate app with
persistent /data/tsdb storage, 180-day/2-GB default retention and configurable
scrape targets. Update the combined telemetry app to 0.2.0 for internal-network
scraping. Cloud retention is unchanged; local history is queried in Prometheus
or a local Grafana data source.

## Grafana dashboard

Import [CTC EcoZenith i360](dashboards/ctc-ecozenith-i360.json) for temperatures,
operating states, domestic hot water and collection health. Choose a Prometheus
data source and the instance/system/device from dropdowns. See
[dashboard instructions and interpretation notes](dashboards/README.md).
