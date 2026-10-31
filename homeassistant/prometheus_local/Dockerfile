FROM prom/prometheus:v3.15.1 AS prometheus
FROM python:3.13-slim-bookworm
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY --from=prometheus /bin/prometheus /usr/local/bin/prometheus
COPY --from=prometheus /bin/promtool /usr/local/bin/promtool
COPY run.py /usr/local/bin/run.py
ENV PYTHONUNBUFFERED=1
ENTRYPOINT ["python3", "/usr/local/bin/run.py"]
