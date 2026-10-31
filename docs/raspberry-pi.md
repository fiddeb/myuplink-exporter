# Raspberry Pi: systemd → Alloy → Grafana Cloud

The exporter emits newline-delimited JSON on stderr. systemd stores it in the
journal; Alloy sends the original lines to Loki and scrapes /probe for metrics.
This guide assumes Raspberry Pi OS/Debian with systemd and Alloy installed from
[Grafana's official package instructions](https://grafana.com/docs/alloy/latest/set-up/install/linux/).
No real credentials are included. Do not overwrite an existing Alloy config.

## Build

On the Pi use `go build -o myuplink-exporter .`. Or cross-compile from your Mac:

```sh
# Raspberry Pi OS 64-bit:
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o myuplink-exporter-pi .
# Raspberry Pi OS 32-bit (including Pi Zero 2 W):
GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 go build -o myuplink-exporter-pi .
scp myuplink-exporter-pi pi@YOUR_PI:/tmp/myuplink-exporter
```

Choose the architecture matching the OS, not just the CPU. Run the remaining
commands on the Pi from the repository directory. For a native build replace
/tmp/myuplink-exporter with your local binary path.

## Exporter service

```sh
sudo install -m 0755 /tmp/myuplink-exporter /usr/local/bin/myuplink-exporter
sudo install -d -m 0700 /etc/myuplink-exporter
sudo install -m 0600 deploy/systemd/exporter.env.example /etc/myuplink-exporter/exporter.env
sudoedit /etc/myuplink-exporter/exporter.env
sudo install -m 0644 deploy/systemd/myuplink-exporter.service /etc/systemd/system/myuplink-exporter.service
sudo systemd-analyze verify /etc/systemd/system/myuplink-exporter.service
sudo systemctl daemon-reload
sudo systemctl enable --now myuplink-exporter
```

Fill in the myUplink Client ID and Client Secret in exporter.env. The system
service manager reads this root-only file before starting the DynamicUser
process. The HTTP endpoint binds only to localhost. Keep background polling
enabled. Confirm operation:

```sh
systemctl status myuplink-exporter --no-pager
journalctl -u myuplink-exporter -n 20 -o cat --no-pager
curl -fsS http://127.0.0.1:8080/probe
```

503 is expected before the first poll finishes. Afterwards probe_success should
be 1. Numeric strVal duplicates are suppressed by default. Opt in with
MYUPLINK_LOG_NUMERIC_STRINGS=true in exporter.env and restart if you need them.
Initial observations recur on exporter restart; there is no durable event state.

## Alloy and Cloud credentials

In your Grafana Cloud stack, copy the Logs push URL/user and the Metrics remote
write URL/user from the connection setup. Create access policy tokens with
logs:write and metrics:write for the relevant stack. Separate tokens are supported.
Use complete URLs including their path. Do not use a Grafana dashboard URL or
myUplink credentials here.

```sh
sudo install -m 0600 deploy/alloy/cloud.env.example /etc/alloy/myuplink-cloud.env
sudoedit /etc/alloy/myuplink-cloud.env
sudo install -d /etc/systemd/system/alloy.service.d
sudo install -m 0644 deploy/alloy/override.conf /etc/systemd/system/alloy.service.d/myuplink.conf
```

Merge deploy/alloy/config.alloy into /etc/alloy/config.alloy. On a fresh Alloy
installation it can be used as the starting config. The drop-in gives Alloy read
access to the system journal via SupplementaryGroups=systemd-journal; the source
filters to _SYSTEMD_UNIT=myuplink-exporter.service. Cloud credentials are loaded
only into Alloy. Remove the prometheus.scrape/remote_write blocks if you only
want logs, and then metrics credentials are unnecessary.

Validate without sending telemetry (subshell loads the root-only environment):

```sh
sudo sh -c 'set -a; . /etc/alloy/myuplink-cloud.env; set +a; alloy validate /etc/alloy/config.alloy'
sudo systemctl daemon-reload
sudo systemctl restart alloy
sudo systemctl enable alloy
systemctl status alloy --no-pager
journalctl -u alloy -n 30 --no-pager
```

Persist Alloy's storage directory (the package service normally configures it).
Journal cursors and the metrics WAL rely on this storage. The journal source
allows an initial lookback of one hour. Retention, rotation and outages can still
cause loss: this is not an exactly-once event delivery system. A Cloud outage
should not stop local exporter collection.

## Verify in Grafana Cloud

Select Loki in Explore, with a recent time range:

```logql
{service_name="myuplink-exporter"} | json
{service_name="myuplink-exporter"} | json | event="point_changed"
{service_name="myuplink-exporter"} | json | level="ERROR"
```

No point_changed lines is normal while states remain stable. Initial
point_observed and startup lines verify the pipeline. Journald's ingestion time
is the Loki event time. observed_at is the exporter's observation time;
source_timestamp is the API's timestamp, possibly last-change time. Do not use it
as Loki's timestamp or infer freshness for every setting from it.

Log labels are service_name, host and Alloy's journal job label. device_id,
parameter_id, alarm_id and value stay inside JSON and are extracted with | json.
No per-device or per-point labels are added to Loki streams.

In the Prometheus data source check:

```promql
up{job="myuplink"}
myuplink_probe_success{job="myuplink"}
time() - myuplink_poll_last_success_timestamp_seconds{job="myuplink"}
```

Expected: up=1, probe_success=1 and last-success age approximately the polling
period plus collection time. Credentials/endpoint errors appear in Alloy logs.
Wait one or two minutes before diagnosing missing remote data.

## Sources and validation limits

Configuration follows Grafana's official references for
[loki.source.journal](https://grafana.com/docs/alloy/latest/reference/components/loki/loki.source.journal/),
[loki.write](https://grafana.com/docs/alloy/latest/reference/components/loki/loki.write/), and
[prometheus.remote_write](https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.remote_write/).
The Cloud endpoints, permissions, Raspberry Pi journal access and live delivery
must be verified in your environment. No Cloud credentials or device changes are
needed for the Go unit tests in docs/testing.md.
