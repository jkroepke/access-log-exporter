#!/bin/sh

set -e

if ! command -v systemctl >/dev/null 2>&1; then
  exit 0
fi

systemctl --system daemon-reload >/dev/null || true

if systemctl is-active --quiet access-log-exporter; then
  systemctl restart access-log-exporter >/dev/null || true
fi

systemd-sysusers /usr/lib/sysusers.d/access-log-exporter.conf >/dev/null || true

if [ -d /etc/access-log-exporter ]; then
  chown -R root:access-log-exporter /etc/access-log-exporter/ >/dev/null || true
fi
