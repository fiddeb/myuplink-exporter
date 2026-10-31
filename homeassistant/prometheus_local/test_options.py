import unittest
from run import configuration


class ConfigurationTests(unittest.TestCase):
    def test_default_storage_and_target(self):
        config, args = configuration({})
        self.assertIn("--storage.tsdb.retention.time=180d", args)
        self.assertIn("--storage.tsdb.retention.size=2GB", args)
        self.assertIn("--storage.tsdb.path=/data/tsdb", args)
        job = config["scrape_configs"][0]
        self.assertEqual(job["metrics_path"], "/probe")
        self.assertEqual(job["static_configs"][0]["targets"], ["local-myuplink-cloud:8080"])
        self.assertNotIn("remote_write", config)

    def test_custom_settings(self):
        config, args = configuration(dict(targets=["other-host:8080"], retention_days=365,
                                          retention_size_gb=4, scrape_interval_seconds=120))
        self.assertEqual(config["global"]["scrape_interval"], "120s")
        self.assertIn("--storage.tsdb.retention.time=365d", args)

    def test_invalid_settings(self):
        for options in [dict(retention_days=0), dict(retention_days=True),
                        dict(targets=[]), dict(targets=["http://host:8080/path"]),
                        dict(targets=["host:99999"]), dict(retention_size_gb="2;bad")]:
            with self.assertRaises(ValueError):
                configuration(options)


if __name__ == "__main__":
    unittest.main()
