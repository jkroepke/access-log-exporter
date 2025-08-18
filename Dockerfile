FROM scratch

ARG TARGETARCH=amd64
ARG TARGETOS=linux

ENTRYPOINT ["/access-log-exporter"]
COPY packaging/etc/access-log-exporter/config.yaml /config.yaml
COPY dist/access-log-exporter_${TARGETOS}_${TARGETARCH}*/access-log-exporter /
