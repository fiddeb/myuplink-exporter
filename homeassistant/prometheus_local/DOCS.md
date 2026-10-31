# Prometheus Local for Home Assistant OS

This is a real Prometheus server with a persistent TSDB, PromQL and local UI.
It complements Alloy/Grafana Cloud: it does not change Cloud retention or store
Loki logs. Default retention is 180 days OR 2 GB of persisted blocks, whichever
is reached first. WAL/head and compaction need additional free space; 2 GB is not
a hard filesystem quota. Retention takes time to accumulate and cannot recover
history already deleted in Cloud.

## Install on your Raspberry Pi 5

In Advanced SSH & Web Terminal:

```sh
git clone --branch improve-monitoring --single-branch https://github.com/fiddeb/myuplink-exporter.git /tmp/myuplink-prometheus-src
# Update the existing telemetry app's source files; /data and saved credentials are unaffected.
cp -R /tmp/myuplink-prometheus-src/homeassistant/myuplink_cloud/. /addons/myuplink_cloud/
# Install the new app folder only if it does not already exist.
test ! -e /addons/prometheus_local && cp -R /tmp/myuplink-prometheus-src/homeassistant/prometheus_local /addons/prometheus_local
```

If the temporary clone already exists, update it with git pull --ff-only instead
of cloning over it. This assumes /addons is the local app directory mounted into
your SSH app, as in the telemetry app installation.

1. Settings → Apps → Install app → top-right menu → Check for updates.
2. Update/rebuild **myUplink Cloud telemetry** to **0.3.0**, then restart it.
   This lets Prometheus reach /probe over the internal app network. No host port
   is published by that app, and its Alloy still scrapes locally as before.
3. Under Local apps, open **Prometheus Local**, then Install.
4. In Configuration, retain defaults or set:

```yaml
targets:
  - local-myuplink-cloud:8080
retention_days: 180
retention_size_gb: 2
scrape_interval_seconds: 60
```

5. Save, Start, and enable Start on boot. Keep Protection mode enabled.
6. Open Web UI, or http://homeassistant.local:9090.

If this UI version has no explicit rebuild action, use the updated version's
Update button; rebuilding the local image is required to pick up run.py changes.
Do not uninstall the telemetry app to update it.

The target uses the telemetry app's internal **Hostname**, normally
local-myuplink-cloud for a local app. If different, copy its Hostname from the
telemetry app Info tab and append :8080. Do not use localhost:8080: that would
point inside the Prometheus app instead of the exporter app.

## Test

In Prometheus, check Status → Targets: job myuplink should be UP within one
scrape interval. Test separately in the query UI:

```promql
up{job="myuplink"}
myuplink_probe_success
myuplink_system_device_connected
myuplink_poll_last_success_timestamp_seconds
```

Expected: up=1 and probe_success=1. If up=0, inspect target error: DNS failures
usually mean a wrong internal hostname; connection refused usually means the
telemetry app has not been rebuilt to 0.2.0 or later or is stopped. If up=1 but
probe_success=0, inspect telemetry app logs for the myUplink API failure.

Check retention via Status → Command-Line Flags or:

```sh
curl -fsS http://homeassistant.local:9090/api/v1/status/flags
```

Expect storage.tsdb.retention.time=180d and retention.size=2GiB (Prometheus normalizes the 2GB flag). The database is
under /data/tsdb and survives restart/update. Verify this by querying a known
historical time range, restarting Prometheus Local, and querying it again.
Uninstalling the app can remove its stored data. Include the app in your backup
plan; avoid frequent large backups on an SD card. Prometheus is not replicated;
an SD failure loses data without backups. An SSD is preferable for sustained use.

The UI/query API has no authentication. Port 9090 is published to your LAN; do not
port-forward it to the internet. No admin API or remote-write receiver is enabled.
You can disable host port mapping in the app Network settings when only other
apps need the query API. They can use http://local-prometheus-local:9090, or the
actual Hostname shown in this app's Info tab.

## Viewing longer history in Grafana

Grafana Cloud still keeps its own 14-day history. To query local 180-day history,
use a local Grafana app with a Prometheus data source URL of
http://local-prometheus-local:9090 (verify the app Hostname). Dashboards using this
data source query your local database. Grafana Cloud cannot automatically query
a private .local address; it would require separate private connectivity.
Do not expose the unauthenticated Prometheus port publicly for that purpose.
The existing Cloud forwarding continues unchanged and does not duplicate scrapes
against the myUplink API: both Alloy and local Prometheus read the same snapshot.

## Validation

The image uses Prometheus v3.15.1 and a generated config, with validated integer
settings and hostname:port targets. Local config tests do not replace an ARM64
image build and installation test on HAOS. Docker/Supervisor are unavailable in
the development environment; no installation or settings changes have been made
to your Home Assistant. The first app install is the full container build test.

Source: https://prometheus.io/docs/prometheus/latest/storage/
