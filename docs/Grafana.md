# Grafana Integration

This guide explains how to visualize access-log-exporter metrics in Grafana.

## Quick Setup

### 1. Import Dashboard

access-log-exporter includes a pre-built Grafana dashboard for visualizing web server metrics.

**Import the dashboard:**

1. Download the dashboard JSON: [grafana-dashboard.json](https://github.com/jkroepke/access-log-exporter/blob/main/contrib/grafana-dashboard.json)
2. In Grafana, go to **Dashboards** → **Import**
3. Upload the JSON file or paste the content
4. Click **Import**

### 2. Dashboard Features

The included dashboard provides:

- **Request Rate**: Requests per second by host and status
- **Response Times**: P50, P95, P99 response time percentiles
- **Error Rates**: 4xx and 5xx error percentages
- **Traffic Volume**: Request/response size metrics
- **Upstream Performance**: Connection and response times (for upstream presets)
- **URI Analytics**: Top endpoints by traffic (for `simple_uri_upstream` preset)

## Demo Setup

For a complete demo environment with Grafana, see the demo configuration in `docs/demo/`:

```bash
cd docs/demo
docker-compose up -d
```

This starts:
- access-log-exporter on port 4040
- Nginx with sample traffic on port 8080
- Prometheus on port 9090
- Grafana on port 3000

The dashboard will be automatically imported and configured.
