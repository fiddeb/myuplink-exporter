"""Run two supervised processes without host access or a Docker socket."""
import json
import logging
from logging.handlers import RotatingFileHandler
import os
from pathlib import Path
import signal
import subprocess
import sys
import threading
from urllib.parse import urlparse


def environment(options):
    required = ("client_id", "client_secret", "loki_url", "loki_user", "loki_token",
                "metrics_url", "metrics_user", "metrics_token", "instance")
    for key in required:
        if not isinstance(options.get(key), str) or not options[key].strip():
            raise ValueError(f"Configure {key} before starting the app")
    for key in ("loki_url", "metrics_url"):
        url = urlparse(options[key])
        if url.scheme != "https" or not url.hostname or url.username or url.password:
            raise ValueError(f"{key} must be an HTTPS URL without embedded credentials")
    exporter = os.environ.copy()
    exporter.update(MYUPLINK_AUTH_CLIENTID=options["client_id"],
                    MYUPLINK_AUTH_CLIENTSECRET=options["client_secret"])
    for key, default, minimum, maximum in (("poll_interval", 60, 30, 3600),
            ("poll_timeout", 55, 10, 600), ("request_interval", 3, 1, 60)):
        value = options.get(key, default)
        if type(value) is not int or not minimum <= value <= maximum:
            raise ValueError(f"Invalid {key}")
        exporter[f"MYUPLINK_{key.upper()}"] = f"{value}s"
    alloy = os.environ.copy()
    alloy["MYUPLINK_INSTANCE"] = options["instance"]
    for prefix in ("LOKI", "METRICS"):
        for suffix in ("URL", "USER", "TOKEN"):
            alloy[f"GRAFANA_CLOUD_{prefix}_{suffix}"] = options[f"{prefix}_{suffix}".lower()]
    return exporter, alloy


def main():
    os.umask(0o077)
    try:
        options = json.loads(Path("/data/options.json").read_text())
        exporter_env, alloy_env = environment(options)
    except (ValueError, OSError):
        # Never print config or exception text from JSON/URL parsers.
        print("Invalid app options: fill all credentials and HTTPS endpoints in Configuration", flush=True)
        return 1
    handler = RotatingFileHandler("/data/exporter.jsonl", maxBytes=5*1024*1024,
                                 backupCount=2, encoding="utf-8")
    handler.setFormatter(logging.Formatter("%(message)s"))
    event_log = logging.getLogger("exporter_file")
    event_log.setLevel(logging.INFO)
    event_log.addHandler(handler)
    stopping = threading.Event()
    signal.signal(signal.SIGTERM, lambda *_: stopping.set())
    signal.signal(signal.SIGINT, lambda *_: stopping.set())
    children = []
    failed = False
    reader = None
    try:
        children.append(subprocess.Popen(["myuplink-exporter", "--server.bind=0.0.0.0:8080",
            "--log.format=json", "--log.time", "--myuplink.collect-alarms"],
            env=exporter_env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True))
        def copy_logs():
            for line in children[0].stdout:
                event_log.info(line.rstrip("\n"))
                sys.stdout.write(line)
                sys.stdout.flush()
        reader = threading.Thread(target=copy_logs, daemon=True)
        reader.start()
        children.append(subprocess.Popen(["alloy", "run", "--server.http.listen-addr=127.0.0.1:12345",
            "--storage.path=/data/alloy", "/etc/myuplink/config.alloy"], env=alloy_env))
        while not stopping.wait(1):
            if any(child.poll() is not None for child in children):
                print("A telemetry process stopped; restart the app and inspect its logs", flush=True)
                failed = True
                break
    except OSError:
        print("Unable to start telemetry processes", flush=True)
        failed = True
    finally:
        for child in children:
            if child.poll() is None:
                child.terminate()
        for child in children:
            try:
                child.wait(timeout=15)
            except subprocess.TimeoutExpired:
                child.kill()
                child.wait()
        if reader is not None:
            reader.join(timeout=5)
        handler.close()
    return int(failed)


if __name__ == "__main__":
    sys.exit(main())
