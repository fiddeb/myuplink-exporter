FROM golang:1.27.3-bookworm AS exporter
WORKDIR /src
# Pin exporter source independently of the local app build context.
RUN git init && git remote add origin https://github.com/fiddeb/myuplink-exporter.git \
    && git fetch --depth=1 origin 40c878176aaedce53b6142ec6091e183f74cd075 \
    && git checkout FETCH_HEAD
RUN CGO_ENABLED=0 go build -trimpath -o /myuplink-exporter .

FROM grafana/alloy:v1.21.3 AS alloy
FROM python:3.13-slim-bookworm
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates libsystemd0 \
    && rm -rf /var/lib/apt/lists/*
COPY --from=exporter /myuplink-exporter /usr/local/bin/myuplink-exporter
COPY --from=alloy /bin/alloy /usr/local/bin/alloy
COPY config.alloy /etc/myuplink/config.alloy
COPY run.py /usr/local/bin/run.py
ENV PYTHONUNBUFFERED=1
ENTRYPOINT ["python3", "/usr/local/bin/run.py"]
