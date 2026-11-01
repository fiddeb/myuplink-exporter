"""Generate a constrained Prometheus config and exec the server as PID 1."""
import json
import os
from pathlib import Path
import re
import sys


def configuration(options):
    defaults = dict(retention_days=180, retention_size_gb=2, scrape_interval_seconds=60)
    ranges = dict(retention_days=(1, 3650), retention_size_gb=(1, 64),
                  scrape_interval_seconds=(15, 3600))
    values = {}
    for key, default in defaults.items():
        value = options.get(key, default)
        low, high = ranges[key]
        if type(value) is not int or not low <= value <= high:
            raise ValueError(f"Invalid {key}")
        values[key] = value
    targets = options.get("targets", ["local-myuplink-cloud:8080"])
    if not isinstance(targets, list) or not targets or len(targets) > 50:
        raise ValueError("Set at least one target")
    for target in targets:
        if not isinstance(target, str) or not re.fullmatch(r"[a-zA-Z0-9.-]+:[0-9]{1,5}", target):
            raise ValueError("Targets must use hostname:port, without a URL or path")
        if not 1 <= int(target.rsplit(":", 1)[1]) <= 65535:
            raise ValueError("Invalid target port")
    config = {
        "global": {"scrape_interval": f"{values['scrape_interval_seconds']}s"},
        "scrape_configs": [
            {"job_name": "myuplink", "metrics_path": "/probe", "scrape_timeout": "5s",
             "static_configs": [{"targets": targets}]},
            {"job_name": "prometheus", "static_configs": [{"targets": ["127.0.0.1:9090"]}]},
        ],
    }
    args = ["prometheus", "--config.file=/data/prometheus.yml", "--storage.tsdb.path=/data/tsdb",
            f"--storage.tsdb.retention.time={values['retention_days']}d",
            f"--storage.tsdb.retention.size={values['retention_size_gb']}GB",
            "--web.listen-address=0.0.0.0:9090"]
    return config, args


def main():
    os.umask(0o077)
    try:
        config, args = configuration(json.loads(Path("/data/options.json").read_text()))
        # JSON is a valid YAML subset; no user string is interpolated into YAML.
        Path("/data/prometheus.yml").write_text(json.dumps(config, indent=2))
        Path("/data/tsdb").mkdir(exist_ok=True)
    except (ValueError, OSError):
        print("Invalid Prometheus options; check targets and retention settings", flush=True)
        return 1
    os.execvp(args[0], args)


if __name__ == "__main__":
    sys.exit(main())
