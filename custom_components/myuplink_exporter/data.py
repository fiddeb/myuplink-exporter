"""Decode the exporter's cached Prometheus snapshot without contacting myUplink."""
import math
import time
from urllib.parse import urlsplit
from prometheus_client.parser import text_string_to_metric_families


def normalize_url(value):
    url = urlsplit(value.strip())
    if url.scheme not in ("http", "https") or not url.hostname or url.username or url.password or url.query or url.fragment or url.path not in ("", "/", "/probe"):
        raise ValueError("Use an HTTP(S) exporter URL without credentials or query parameters")
    return f"{url.scheme}://{url.netloc}/probe"


def parse_snapshot(text, max_age=300, now=None):
    now = time.time() if now is None else now
    samples = [s for family in text_string_to_metric_families(text) for s in family.samples]
    def scalar(name):
        return next((s.value for s in samples if s.name == name and not s.labels), None)
    stamp = scalar("myuplink_poll_last_success_timestamp_seconds")
    if scalar("myuplink_probe_success") != 1 or stamp is None or not math.isfinite(stamp) or not 0 <= now - stamp <= max_age:
        raise ValueError("Exporter snapshot failed or is stale; background polling is required")
    devices, values, enums, timestamps = {}, {}, {}, {}
    for s in samples:
        if not math.isfinite(s.value):
            continue
        labels = s.labels
        system, device = labels.get("systemID"), labels.get("deviceID")
        if not system:
            continue
        if device:
            key = (system, device)
            if s.name == "myuplink_system_device_info":
                devices[key] = dict(labels)
            elif s.name == "myuplink_system_device_point":
                values[(system, device, labels["parameterID"])] = dict(value=s.value, name=labels.get("parameterName", labels["parameterID"]), unit=labels.get("parameterUnit", ""), binary=False)
            elif s.name == "myuplink_system_device_point_timestamp_seconds":
                timestamps[(system, device, labels["parameterID"])] = s.value
            elif s.name == "myuplink_system_device_point_enum" and s.value == 1:
                enums[(system, device, labels["parameterID"])] = labels["valueText"]
            elif s.name == "myuplink_system_device_connected":
                values[(system, device, "connected")] = dict(value=s.value == 1, name="Connected", binary=True, device_class="connectivity")
        elif s.name == "myuplink_system_has_alarm":
            devices[(system, "system")] = dict(deviceName="myUplink system")
            values[(system, "system", "alarm")] = dict(value=s.value == 1, name="Active alarm", binary=True, device_class="problem")
    # CTC EcoZenith has been observed returning invalid measurements of -32768.
    # Exclude these points so existing entities become unavailable, not numeric.
    values = {key: point for key, point in values.items()
              if not (devices.get(key[:2], {}).get("deviceName", "").startswith("CTC EcoZenith")
                      and point["value"] == -32768)}
    for key, point in values.items():
        if point.get("unit") == "kW" and key[2] in ("62168", "62361"):
            point["name"] = {"62168": "Auxiliary heater power", "62361": "Immersion heater power"}[key[2]]
    for key, value in enums.items():
        if key in values:
            values[key].update(value=value, unit="")
    # Only the CTC RPS parameter has a verified compressor-running interpretation.
    for key, point in list(values.items()):
        if key[2] == "62193" and (point.get("unit") == "RPS" or devices.get(key[:2], {}).get("deviceName", "").startswith("CTC EcoZenith")) and isinstance(point["value"], (int, float)):
            values[(*key[:2], "compressor_running")] = dict(value=point["value"] > 0, name="Compressor running", binary=True, device_class="running")
    return dict(devices=devices, values=values, timestamps=timestamps, observed_at=stamp)
