# CTC EcoZenith i360 dashboard

Import `ctc-ecozenith-i360.json` through Grafana → Dashboards → New → Import →
Upload dashboard JSON file, then Import. Choose your Prometheus data source from
the dashboard's Prometheus dropdown, then Instance, System and Device. These
variables populate from the exported point metrics; no private IDs are embedded.
If the dropdowns are empty, first confirm `myuplink_system_device_point` in Explore
using that data source and expand the time range if needed.

Use the Grafana Cloud Metrics source for Cloud history, or your local Prometheus
source for local retention. A Cloud Grafana cannot reach a private Home Assistant
.local address without separate private connectivity. A local Grafana app can use
`http://local-prometheus-local:9090` (verify its internal Hostname).

## Panels

25 panels cover connection and system alarm status, collection success, outdoor
and water temperatures, tank capacity, enum status text, heating setpoint/return,
compressor RPS, pump %, refrigerant temperatures/pressures, degree minutes,
instantaneous auxiliary power, poll age and source timestamps.

The layout uses only built-in Grafana panel types and a selectable Prometheus
data source. No plugin, Loki data source, or dashboard provisioning permissions
are required. Loki can be queried separately with
`{service_name="myuplink-exporter"} | json`.

## Interpretation

Parameter IDs were checked against the supplied live CTC sample. Meaning and
units retain myUplink v2 semantics and need comparison with your CTC display:

- 62003 is deliberately excluded because the sample reports -32768 °C.
- 61590/61591 are not interpreted as instantaneous power consumption.
- 62168/62361 are shown separately, not added together.
- No COP or energy integration is invented from incomplete data.
- 62203 is labeled by its generic API name, not assumed to be a physical room sensor.
- Active enum text is shown only when API enum mappings are available.
- System alarm panels cover the entire selected system, not only the chosen device.
- API collection/last-success panels cover the entire instance.
- Missing data displays as missing; it is not replaced by zero.
- The source timestamp diagnostic may reflect last change rather than last poll;
  it is not used as a definitive device-freshness alarm.

Validation: JSON structure and panel layout checked; all 35 PromQL target
expressions (with template values substituted) parsed with Prometheus promtool;
point IDs checked against the supplied metrics sample. The dashboard has not yet
been imported/rendered in your live Grafana. On import, verify data source and
variable selections first, then compare current values with the manufacturer app.
