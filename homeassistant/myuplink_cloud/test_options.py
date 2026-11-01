import unittest
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
