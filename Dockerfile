FROM scratch
ENTRYPOINT ["/access-log-exporter"]
COPY packaging/etc/access-log-exporter/config.yaml /config.yaml
COPY access-log-exporter /
