FROM gcr.io/distroless/static-debian13:nonroot
ARG TARGETPLATFORM
ENTRYPOINT ["/access-log-exporter"]
COPY packaging/etc/access-log-exporter/config.yaml /config.yaml
COPY $TARGETPLATFORM/access-log-exporter /
