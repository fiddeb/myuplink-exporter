# Dashboard languages

| Layout | English | Swedish |
| --- | --- | --- |
| Dynamic tabs (schema v2) | [ctc-ecozenith-i360.dynamic.en.json](ctc-ecozenith-i360.dynamic.en.json) | [ctc-ecozenith-i360.dynamic.json](ctc-ecozenith-i360.dynamic.json) |
| Classic sections | [ctc-ecozenith-i360.en.json](ctc-ecozenith-i360.en.json) | [ctc-ecozenith-i360.json](ctc-ecozenith-i360.json) |

English uses UID `ctc-myuplink-overview-en`; Swedish uses
`ctc-myuplink-overview`. They can coexist. The two layouts of each language share
their UID, so importing a different layout replaces that language's dashboard.
English translates titles, descriptions, legends, state mappings and LogQL display
text. Query selectors, parameter IDs and raw API values/enum text are unchanged.
Keep both language variants in sync when changing panels or queries.

# Dashboard layouts

## Canvas system overview

The overview includes a native Canvas panel with an outdoor unit, EcoZenith tank,
and house. Fifteen instant queries reuse the existing point selectors: outdoor
temperature, pump inlet/outlet, tank temperature, hot-water capacity, heating
flow/return/setpoint, compressor RPS, charge-pump percentage, two separate
electrical-power readings, indoor temperature, room setpoint, and degree minutes. No additional myUplink API calls
are triggered: Grafana queries the existing Prometheus samples.

The fan is blue when reported compressor speed is positive and gray at zero.
It does not animate or represent measured water flow. Pipes are schematic:
amber denotes supply and blue denotes return, not health. Tank capacity is the
reported percentage, not a measured physical water level. The two electrical
power readings are not summed. Missing values display as an em dash; existing
values may still be stale, so check collection health above the Canvas.

Click readings or the links below each component for details. Dynamic versions
link to the relevant tab; classic versions open the corresponding detail panel.
Links preserve the selected variables and time range. Canvas uses native shapes
and metric fields, with no remote image dependencies or control/API actions.
Zoom-to-content fits the drawing to the available panel size; the full diagram
is easiest to read on a desktop or tablet. Pan/zoom is enabled for closer viewing.

To update the bilingual Canvas consistently, edit `scripts/build_canvas.py` and
run `python scripts/build_canvas.py` from the repository. It updates panel 500
in all four dashboard files and the overview layout. It leaves data-source
variables and other detail panels intact. Test against your Grafana version:
the live Canvas was checked in Grafana Cloud 13.3, including actual metric values.

## Colors

Ordinary stat values use neutral theme text (white in dark mode). Hot-water
capacity gauges use white, without implying an unverified healthy range.
Connection, system alarms and API success retain explicit green/red mappings.
Active alarm count is green at zero and red from one. Poll age uses the existing
green/orange/red thresholds at 180 and 300 seconds. Timeline activity is blue,
inactivity gray: running is an operating state, not a health assessment.
Time-series colors still distinguish measurements. These choices apply to both
languages and layouts. Repository variables remain selectable; when updating
a customized live dashboard, apply the visual changes to its existing JSON to
preserve its data sources, variable visibility and selected values.

Use **ctc-ecozenith-i360.dynamic.json** for Grafana with Dynamic dashboards and
schema v2. It is the layout used in the live Grafana Cloud dashboard:

- Overview: connection, alarm status, system mode, poll age, outdoor/flow/tank
  temperatures, hot-water capacity and the operating state timeline.
- Space heating / Husvärme: flow, return, setpoint, degree minutes and outdoor/room values.
- Hot water / Varmvatten: temperature, capacity, setpoints and related Loki events.
- Heat pump / Värmepump: pump status, compressor speed, delta-T, auxiliary power and refrigerant.
- Logs & troubleshooting / Loggar & felsökning: observations/changes, alarms, exporter errors and collection health.

Time range and data-source/device variables are shared across tabs. Import the
v2 resource JSON through Grafana Import, or paste it into Edit → Code. Select
Prometheus and Loki, then Instance, System and Device. The file contains no
private instance, system or device defaults. Its UID matches the classic layout;
importing it replaces that layout for the same dashboard.

The classic `ctc-ecozenith-i360.json` remains available for Grafana without v2
layout support; it uses sections instead of tabs.

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

Space heating also shows the reported heating-curve slope (`61513`) and
adjustment (`61517`). These settings do not reconstruct the complete effective
curve, which can include additional control corrections. The heat-pump section
shows reported compressor runtime over the last 24 hours (`62234`), converted
from minutes to hours. This rolling value is not a cumulative counter.

Parameter IDs were checked against the supplied live CTC sample. Meaning and
units retain myUplink v2 semantics and need comparison with your CTC display:

- 62003 is deliberately excluded because the sample reports -32768 °C.
- 61590/61591 are not interpreted as instantaneous power consumption.
- 62168/62361 are shown separately, not added together.
- No COP or energy integration is invented from incomplete data.
- 62203 is indoor temperature and 61509 the room setpoint, confirmed by the owner
  for this installation. Verify this mapping on other installations.
- Active enum text is shown only when API enum mappings are available.
- System alarm panels cover the entire selected system, not only the chosen device.
- API collection/last-success panels cover the entire instance.
- Missing data displays as missing; it is not replaced by zero.
- The source timestamp diagnostic may reflect last change rather than last poll;
  it is not used as a definitive device-freshness alarm.

Validation: original PromQL targets retain their validated expressions. The
revised layout and data-source references are checked locally. The primary
formatted LogQL query is checked against actual Grafana Cloud logs.

## Operating timeline

The overview includes four state-timeline lanes: compressor (62193 RPS > 0),
heating mode (62005 enum HC / HC mixing / DHW/HC), hot-water mode (62005 enum DHW /
DHW/HC), and defrost (62017 enum Defrost). Enum values are selected by their API
text, not invented numeric codes. Mode selection is not proof of delivered heat.
Missing samples are not converted to stopped. Check collection/device health
separately: myUplink can report a last-known value. Queries request a 15-second minimum step and 5000 points,
but longer time ranges can use a larger step. Actual resolution is limited by
myUplink updates and polling, so this is not a precise compressor-cycle counter.
