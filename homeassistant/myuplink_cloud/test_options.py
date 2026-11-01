import unittest
import json
from unittest.mock import MagicMock, patch
import run
from run import environment


class OptionsTests(unittest.TestCase):
    def options(self):
        return dict(client_id="id", client_secret="secret", loki_url="https://logs.example/loki/api/v1/push",
                    loki_user="user", loki_token="logs-secret", metrics_url="https://metrics.example/api/prom/push",
                    metrics_user="user", metrics_token="metrics-secret", instance="homeassistant")

    def test_separate_credentials(self):
        exporter, alloy = environment(self.options())
        self.assertEqual(exporter["MYUPLINK_AUTH_CLIENTSECRET"], "secret")
        self.assertEqual(exporter["MYUPLINK_POLL_INTERVAL"], "60s")
        self.assertEqual(alloy["GRAFANA_CLOUD_LOKI_TOKEN"], "logs-secret")
        self.assertNotIn("GRAFANA_CLOUD_LOKI_TOKEN", exporter)
        self.assertNotIn("MYUPLINK_AUTH_CLIENTSECRET", alloy)

    def test_exporter_without_cloud_credentials(self):
        options = dict(client_id="id", client_secret="secret", instance="homeassistant", cloud_enabled=False)
        exporter, alloy = environment(options)
        self.assertIsNone(alloy)
        self.assertEqual(exporter["MYUPLINK_POLL_INTERVAL"], "60s")

    def test_cloud_requires_credentials_and_boolean(self):
        for change in (dict(cloud_enabled=True, metrics_token=""), dict(cloud_enabled="false")):
            options = self.options()
            options.update(change)
            with self.assertRaises(ValueError):
                environment(options)

    def test_processes_follow_cloud_setting(self):
        for enabled in (False, True):
            options = self.options()
            options["cloud_enabled"] = enabled
            process = MagicMock()
            process.stdout = []
            process.poll.return_value = None
            with patch("run.Path.read_text", return_value=json.dumps(options)), \
                 patch("run.RotatingFileHandler"), patch("run.os.umask"), \
                 patch("run.signal.signal"), patch("run.threading.Thread"), patch("run.threading.Event") as event, \
                 patch("run.subprocess.Popen", return_value=process) as popen:
                event.return_value.wait.return_value = True
                self.assertEqual(run.main(), 0)
                self.assertEqual(popen.call_count, 2 if enabled else 1)
                self.assertEqual(popen.call_args_list[0].args[0][0], "myuplink-exporter")
                if enabled:
                    self.assertEqual(popen.call_args_list[1].args[0][0], "alloy")

    def test_reject_invalid_options(self):
        for change in (dict(client_secret=""), dict(loki_url="http://example.com"),
                       dict(loki_url="https://user:secret@example.com"), dict(poll_timeout=0),
                       dict(request_interval="3; echo secret")):
            with self.subTest(change=list(change)):
                options = self.options()
                options.update(change)
                with self.assertRaises(ValueError):
                    environment(options)


if __name__ == "__main__":
    unittest.main()
