FROM scratch

ENTRYPOINT ["/access-log-exporter"]
COPY dist/access-log-exporter_${TARGETOS}_${TARGETARCH}*/access-log-exporter /
COPY access-log-exporter /
