# Home Assistant OS installation

This local app bundles the myUplink exporter and Grafana Alloy. No changes to the
HAOS host, disabled Protection mode, Docker socket, or Home Assistant API token
are required. Supported: aarch64 (your raspberrypi5-64) and amd64.

## Install a local app

Open Advanced SSH & Web Terminal. The local apps folder is usually mounted as
/addons in this app. Run:

```sh
test -d /addons || { echo '/addons is unavailable: use Samba to access local_apps'; exit 1; }
git clone --branch improve-monitoring --single-branch https://github.com/fiddeb/myuplink-exporter.git /tmp/myuplink-exporter-src
test ! -e /addons/myuplink_cloud && cp -R /tmp/myuplink-exporter-src/homeassistant/myuplink_cloud /addons/myuplink_cloud
```

If the temporary clone path already exists, use a different temporary path or
update that checkout. Do not overwrite an already installed app directory without
reviewing the update. Samba may expose the same folder as local_apps or addons.

1. Settings → Apps → Install app (older UI: Settings → Add-ons → Add-on store).
2. Use the top-right menu to check for updates/reload the store.
3. Under Local apps, open **myUplink Cloud telemetry** and click Install.
4. Installation builds the exporter from a pinned commit and downloads Alloy.
   Allow several minutes and internet access; inspect Supervisor logs on failure.
5. Open Configuration and fill in every credential/endpoint field below, then Save.
6. Click Start, enable Start on boot and optionally Watchdog. Keep Protection mode on.
7. Inspect the app's Log tab.

This is a local app build, not a prebuilt signed release. The pinned exporter
source is ab61837c50d34e339fe47df7eb9f019803044a50. Alloy is v1.21.3. Future source
updates need a reviewed Dockerfile pin and app version bump, followed by rebuild.

## Configuration

| Field | Value |
| --- | --- |
| client_id | myUplink application's Client Identifier |
| client_secret | myUplink application's Client Secret |
| loki_url | Full Grafana Cloud Logs push URL, ending /loki/api/v1/push |
| loki_user | Grafana Cloud Logs username/instance ID |
| loki_token | Access policy token with logs:write for your stack |
| metrics_url | Full Grafana Cloud Metrics remote write URL from your stack |
| metrics_user | Grafana Cloud Metrics username/instance ID |
| metrics_token | Access policy token with metrics:write for your stack |
| instance | Stable label such as homeassistant |
| poll_interval | Seconds between completed polls; default 60 |
| poll_timeout | Poll deadline in seconds; default 55 |
| request_interval | Minimum spacing between API calls in seconds; default 3 |

Example structure (replace all placeholders in the app UI):

```yaml
client_id: "YOUR_MYUPLINK_CLIENT_ID"
client_secret: "YOUR_MYUPLINK_CLIENT_SECRET"
loki_url: "https://YOUR_LOGS_HOST/loki/api/v1/push"
loki_user: "YOUR_LOGS_USER"
loki_token: "YOUR_LOGS_WRITE_TOKEN"
metrics_url: "https://YOUR_METRICS_HOST/api/prom/push"
metrics_user: "YOUR_METRICS_USER"
metrics_token: "YOUR_METRICS_WRITE_TOKEN"
instance: homeassistant
poll_interval: 60
poll_timeout: 55
request_interval: 3
```

Use the exact endpoints from Grafana Cloud Connections, not a dashboard URL.
Both log and metrics forwarding are enabled in this version; all fields are
required. These secrets stay in the app's protected options and process environment.
Keep backups of the app private; they may include options and telemetry.

## Verify

The Log tab should show exporter startup, point_observed for text states, and
Alloy starting. Numeric string duplicates are filtered. A 503/up=0 during the
first poll is expected. After one or two minutes, Grafana Cloud Explore:

Loki:

```logql
{service_name="myuplink-exporter", instance="homeassistant"} | json
```

Prometheus:

```promql
up{job="myuplink",instance="homeassistant"}
myuplink_probe_success{job="myuplink",instance="homeassistant"}
time() - myuplink_poll_last_success_timestamp_seconds{instance="homeassistant"}
```

Expected: up=1, probe_success=1, recent last-success. No changed events is normal
when text states remain stable. Cloud authentication/endpoint errors appear in
the app Log tab. Device faults must not be triggered for testing.

Exporter JSON is copied to /data/exporter.jsonl (5 MiB, two rotated backups) and
the app Log tab. Alloy reads it with source.file, not host journald. Cursor/WAL
state lives in /data/alloy and survives restarts. No external metrics or Alloy UI
ports are exposed. Log ingestion time is used; source_timestamp stays in JSON.
Rotation/outages can still lose events; there is no exactly-once guarantee.
Initial observations repeat on restart. The child processes are stopped together;
if either exits the app stops rather than silently running half a pipeline.

## Validation status

Go tests/race detector, Python option tests and Alloy config validation can run
off-device. This environment does not have Docker: the full image build, Supervisor
installation, ARM64 runtime and real Grafana Cloud delivery still need the first
installation test on your Pi. Nothing has been installed or changed in your Home
Assistant by creating these files.
