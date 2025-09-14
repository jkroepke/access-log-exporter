FROM scratch
ARG TARGETPLATFORM
ENTRYPOINT ["/access-log-exporter"]
COPY packaging/etc/access-log-exporter/config.yaml /config.yaml
COPY $TARGETPLATFORM/access-log-exporter /
