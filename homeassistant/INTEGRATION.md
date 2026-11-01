# Home Assistant integration

The `myuplink_exporter` custom integration reads the Go exporter's cached
Prometheus `/probe` endpoint. It does not call myUplink, require MQTT, query a
Prometheus server, or store Cloud credentials. The exporter still needs myUplink
credentials and internet access. This is a read-only custom integration, not an
integration included in Home Assistant Core or a HACS default repository.

Tested with Home Assistant Core **2026.10.4** and Python **3.14**.

## Install

1. Install/update the local `myuplink_cloud` app to **0.4.1**, following its
   [app instructions](myuplink_cloud/DOCS.md). Start the app and wait for a
   successful poll. Keep `cloud_enabled: true` for existing Grafana Cloud
   telemetry, or set it to `false` to run the exporter alone.
2. Copy `custom_components/myuplink_exporter` from this repository into
   `/config/custom_components/myuplink_exporter` in your **Home Assistant Core
   configuration directory** (using Samba, Studio Code Server or a terminal
   with that directory mounted). The result must contain `manifest.json`
   directly inside that folder. Do not put it in `/addons`.
3. Restart Home Assistant Core.
4. Settings → Devices & services → Add integration → **myUplink Exporter**.
5. Enter `http://local-myuplink-cloud:8080`. This is the internal hostname of
   the locally installed app, available from Core on the Supervisor network.
   If you installed the exporter elsewhere, use its reachable HTTP(S) address.
   Do not use `localhost`: that would address Core, not the app.
6. Leave maximum snapshot age at 300 seconds for the default polling schedule.
   With longer intervals, choose a threshold greater than `poll_interval +
   poll_timeout`, allowing time for a retry. It is configurable during setup.

Example terminal installation from an existing repository checkout, with Core's
configuration mounted at `/config`:

```sh
mkdir -p /config/custom_components
cp -R custom_components/myuplink_exporter /config/custom_components/
ha core restart
```

No host port needs exposing on HAOS. Outside HAOS, restrict access to the
exporter to trusted clients; its endpoint contains device identifiers and has
no built-in authentication. Use a trusted HTTPS proxy if crossing networks.

## Entities and availability

- Each API device is grouped by system ID and device ID. A separate system
  device holds the system-wide active-alarm binary sensor.
- Numeric parameters become sensors, preserving API units and names. Active
  enum values become text states. Pure string log events are not reconstructed
  from Loki and do not become entities in this version.
- Connected state and verified CTC compressor RPS (`62193`) become binary sensors.
- New parameters/devices are discovered during subsequent polls. Missing
  parameters become unavailable. Stable IDs survive exporter restarts.
- A failed HTTP request, failed probe, missing polling timestamp, or expired
  successful-poll timestamp makes entities unavailable. Recovery is automatic.
- A disconnected device keeps its connection binary sensor available as `off`;
  its measurement entities become unavailable.
- Data is polled locally every 30 seconds. Freshness checks concern the API
  collection, not the age of unchanged individual source values.
- Known temperature/power/current/voltage/frequency/pressure units receive
  measurement metadata. Energy-like fields are not automatically marked as
  cumulative energy: their semantics must be verified first. Rolling 24-hour
  runtime remains a gauge. Auxiliary heater power is not whole-system power.

The Go app remains usable without this integration. Local Prometheus and Alloy
can scrape the same snapshot independently. Changing `cloud_enabled` requires
an app restart and leaves its local endpoint available. The image still bundles
Alloy, but the process is not started when Cloud export is disabled.

## Test on your installation

1. With `cloud_enabled: false`, clear Cloud fields and start the app. Verify a
   successful poll, no Alloy process messages, and entities appearing in Core.
2. Compare outdoor/indoor temperatures, mode and compressor speed against the
   same `/probe` snapshot (or Grafana when Cloud is enabled). Allow 30 seconds
   for the integration refresh; Cloud adds its own scrape/delivery latency.
3. Stop the app. Within the next local refresh and request timeout, entities
   should become `unavailable`. Restart it: they should recover without new IDs.
4. To test stale data without interrupting the pump, run a test HTTP fixture
   serving `/probe` with `myuplink_probe_success 1` and a last-success timestamp
   older than `max_age`. Setup must reject it; an already configured integration
   must become unavailable. Do not change the system clock.
5. Set `cloud_enabled: true`, restore valid metrics/logs endpoints and tokens,
   and restart. Confirm both Grafana telemetry and Home Assistant entities work.
6. Reload the integration and restart Core. Confirm no duplicate entities.

Automated tests cover snapshot parsing, freshness, enums, multiple systems,
entity availability, discovery and app options. A live HAOS installation test
is still required before treating this first custom-integration release as
production-validated.

Run the automated checks locally (Python 3.14):

```sh
python -m venv .venv
. .venv/bin/activate
pip install homeassistant==2026.10.4 prometheus-client==0.21.0 pytest pytest-asyncio
python -m pytest tests/homeassistant -v
python -m unittest discover -s homeassistant/myuplink_cloud -v
```

## Dashboard

`dashboard-ctc.json` is a native Home Assistant dashboard (JSON is valid YAML).
Create an empty dashboard and paste the file into its Raw configuration editor.
It has Swedish Overview, Space heating, Hot water and Heat pump views using
built-in tile, history, entities and logbook cards. No HACS frontend cards are
required. Verify entity IDs against your entity registry; the sample uses the
IDs created for a first CTC EcoZenith i360 installation. History and logbook use
Home Assistant Recorder, not the separate Prometheus database.

Integration 0.1.2 ignores CTC EcoZenith numeric values of -32768 observed during
mode transitions; existing entities become unavailable until valid values
return. It does not remove invalid samples already stored in Recorder.

### Last valid return temperature (integration 0.1.3)

For CTC EcoZenith, two separate entities retain the last valid return temperature
and its source timestamp. The overview displays these when current return is
unavailable. Historical entities have no measurement state class and do not
replace the current sensor or exporter metrics. The timestamp uses the source
measurement time, falling back to the exporter's successful poll time when absent.
Values are retained in memory across missing points and connection failures,
but reset on integration reload/Home Assistant restart until a valid reading arrives.
