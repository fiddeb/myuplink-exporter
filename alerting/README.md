# myUplink alerts

`grafana-myuplink.json` is a Grafana file-provisioning export of the three
Grafana-managed rules configured in Cloud. It is not dashboard JSON or a
Prometheus rule file. The live rules are in folder **Hemma**, group **myuplink**,
evaluated every minute, using contact point **Cloud alerts** (Grafana IRM).

| Rule | Condition | Pending |
| --- | --- | --- |
| Aktivt värmepumpslarm | System alarm flag > 0 | 2m |
| Värmepump offline | Device connected = 0 | 10m |
| Insamling saknas eller är för gammal | Last successful poll age > 300s, missing metric, or query failure | 5m |

The first two rules keep their last state on whole-query No Data/Error.
The collection rule uses `or vector(600)` to detect complete metric loss; 600
is a sentinel, not a measured age. Its No Data/Error handling is Alerting.
The query intentionally aggregates away labels so the missing-data fallback
retains the same alert identity. It monitors only `instance="homeassistant"`.
Use separate collection rules for additional expected instances.

A stopped poll normally fires after roughly ten minutes (age over five
minutes, then five minutes pending). Complete telemetry loss also depends on
Prometheus staleness/lookback. Evaluation alignment and notification grouping
can add delay. A quiet point's source timestamp is not used for freshness:
it can represent the last value change.

These alerts do not prove that every device/point is present or that Loki
delivery works. Grafana Cloud must itself be functioning to evaluate and send.
The system alarm flag is system-wide, not a diagnosis of a specific component.

## Restore or reuse

The export preserves existing rule UIDs; avoid creating duplicate rules.
For another installation adapt datasource UID `grafanacloud-prom`, instance,
folder, receiver, and dashboard links first. The contact point must exist.
Descriptions use `/d/ctc-myuplink-overview`; set an absolute URL for notifications.
Self-hosted Grafana supports this file under `provisioning/alerting/`.
Grafana Cloud cannot read your local provisioning directory: recreate through
the UI or translate the export for the supported provisioning API/Terraform.
Do not send this file directly to a dashboard import or assume its schema is
the same as the alert-rule API request schema.

## Test

1. Open Alerting → Alert rules → Hemma → myuplink. All three should become
   **Normal** after their first evaluation. Preview each rule to verify current
   values: alarm/offline 0, successful-poll age normally below 300 seconds.
2. Open Notification configuration → Cloud alerts → Test. Send a test only
   when ready to notify the configured IRM recipients, and verify delivery
   through the IRM escalation path. A successful rule preview does not test
   delivery. No test notification was sent during setup.
3. For an end-to-end outage test, stop only the **myUplink exporter/Alloy app**
   in Home Assistant. Leave the heat pump and Home Assistant running. Expect
   the collection rule to pass through Pending to Alerting after approximately
   ten minutes, with timing depending on staleness and notification grouping.
   Verify receipt, restart the app, and verify Normal/resolved after fresh data.
   Monitoring and logs are unavailable while that app is stopped.
4. Do not create a real pump fault to test alarm/offline alerts. Use an isolated
   test rule/contact point with synthetic values if their pending transitions
   need an end-to-end test.

Live previews were checked during setup; notification delivery and deliberate
outage tests remain manual. Temperature and cycling alerts need agreed limits
and a representative baseline before enabling them.
