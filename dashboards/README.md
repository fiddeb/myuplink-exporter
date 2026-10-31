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

Panels are grouped into Driftöversikt, Husvärme, Varmvatten, Värmepump och
tillsats, and Larm och felsökning. Select both Prometheus and Loki data sources.
Log panels format JSON at query time, preserving fields for drilldown and working
with logs already stored before the Alloy update. Startup observations are
explicitly distinguished from changes. System alarm history is system-wide;
exporter warnings/errors are instance-wide. Log panels show at most 200 lines.

A quiet period can legitimately have no logs: unchanged values are not repeated.
Expand the time range to include startup, or wait for a real state change.
Prometheus and Loki may have different retention even on the same dashboard.

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

Validation: original PromQL targets retain their validated expressions. The
revised layout and data-source references are checked locally. The primary
formatted LogQL query is checked against actual Grafana Cloud logs.
