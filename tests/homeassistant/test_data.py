"""Snapshot contract tests, independent of Home Assistant installation."""
import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("exporter_data", Path(__file__).parents[2] / "custom_components/myuplink_exporter/data.py")
data = importlib.util.module_from_spec(spec)
spec.loader.exec_module(data)

HEADER = 'myuplink_probe_success 1\nmyuplink_poll_last_success_timestamp_seconds 1000\n'
POINTS = '''myuplink_system_device_info{systemID="s",deviceID="d",deviceName="CTC"} 1
myuplink_system_device_connected{systemID="s",deviceID="d"} 1
myuplink_system_has_alarm{systemID="s"} 0
myuplink_system_device_point{systemID="s",deviceID="d",parameterID="62193",parameterName="Compressor",parameterUnit="RPS"} 23
myuplink_system_device_point{systemID="s",deviceID="d",parameterID="mode",parameterName="Mode",parameterUnit=""} 2
myuplink_system_device_point_enum{systemID="s",deviceID="d",parameterID="mode",valueText="HC"} 1
myuplink_system_device_point_enum{systemID="s",deviceID="d",parameterID="mode",valueText="DHW"} 0
myuplink_system_device_point{systemID="s2",deviceID="d",parameterID="62193",parameterName="Compressor",parameterUnit="RPS"} 0
'''


class SnapshotTests(unittest.TestCase):
    def test_states_units_and_multiple_systems(self):
        result = data.parse_snapshot(HEADER + POINTS, now=1010)
        values = result['values']
        self.assertEqual(values['s', 'd', '62193']['value'], 23)
        self.assertEqual(values['s', 'd', 'mode']['value'], 'HC')
        self.assertTrue(values['s', 'd', 'compressor_running']['value'])
        self.assertFalse(values['s2', 'd', 'compressor_running']['value'])
        self.assertFalse(values['s', 'system', 'alarm']['value'])

    def test_ctc_missing_value_not_exposed_as_measurement_or_running(self):
        text = (HEADER + POINTS).replace('deviceName="CTC"', 'deviceName="CTC EcoZenith i360"').replace('} 23', '} -32768')
        values = data.parse_snapshot(text, now=1001)['values']
        self.assertNotIn(('s', 'd', '62193'), values)
        self.assertNotIn(('s', 'd', 'compressor_running'), values)
        self.assertIn(('s', 'd', 'connected'), values)

    def test_ctc_compressor_with_empty_api_unit(self):
        text = (HEADER + POINTS).replace('deviceName="CTC"', 'deviceName="CTC EcoZenith i360"').replace('parameterUnit="RPS"', 'parameterUnit=""')
        result = data.parse_snapshot(text, now=1001)
        self.assertTrue(result['values']['s', 'd', 'compressor_running']['value'])
        self.assertNotIn(('s2', 'd', 'compressor_running'), result['values'])

    def test_failed_stale_missing_and_future_poll(self):
        for text, now in [(HEADER, 1301), (HEADER, 999),
                          (HEADER.replace('success 1', 'success 0'), 1001),
                          ('myuplink_probe_success 1\n', 1001),
                          (HEADER.replace('1000', 'NaN'), 1001)]:
            with self.subTest(text=text, now=now), self.assertRaises(ValueError):
                data.parse_snapshot(text, now=now)

    def test_nonfinite_point_not_exposed(self):
        result = data.parse_snapshot(HEADER + POINTS.replace('} 23', '} NaN'), now=1001)
        self.assertNotIn(('s', 'd', '62193'), result['values'])

    def test_url(self):
        self.assertEqual(data.normalize_url('http://example:8080/'), 'http://example:8080/probe')
        for url in ['ftp://host', 'http://user:secret@host', 'http://host/metrics', 'http://host?token=x']:
            with self.subTest(url=url), self.assertRaises(ValueError):
                data.normalize_url(url)
