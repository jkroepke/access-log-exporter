#!/bin/sh

set -e

if ! command -v systemctl >/dev/null 2>&1; then
  exit 0
fi

systemctl --system daemon-reload >/dev/null || true

if systemctl is-active --quiet prometheus-nginxlog-exporter; then
  systemctl restart prometheus-nginxlog-exporter >/dev/null || true
fi

systemd-sysusers /usr/lib/sysusers.d/prometheus-nginxlog-exporter.conf >/dev/null || true

if [ -d /etc/prometheus-nginxlog-exporter ]; then
  chown -R root:prometheus-nginxlog-exporter /etc/prometheus-nginxlog-exporter/ >/dev/null || true
fi
