# Prometheus Integration

This guide explains how to integrate access-log-exporter with Prometheus.

## Quick Setup

### 1. Start access-log-exporter

```bash
access-log-exporter --preset simple
```

By default, metrics are exposed on `http://localhost:4040/metrics`

### 2. Configure Prometheus

Add this to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'access-log-exporter'
    static_configs:
      - targets: ['localhost:4040']
    scrape_interval: 30s
```

### 3. Verify

Check Prometheus targets at `http://your-prometheus:9090/targets`

## Available Metrics

### Basic Metrics (simple preset)
- `http_requests_total` - Request counter with labels: host, method, status
- `http_request_duration_seconds` - Response time histogram
- `http_request_size_bytes` - Request size histogram
- `http_response_size_bytes` - Response size histogram

### Upstream Metrics (simple_upstream preset)
- `http_upstream_connect_duration_seconds` - Upstream connection time
- `http_upstream_header_duration_seconds` - Upstream header time
- `http_upstream_request_duration_seconds` - Upstream response time

### URI Tracking (simple_uri_upstream preset)
All metrics include `request_uri` label with path normalization:

```prometheus
http_requests_total{host="example.com",method="GET",status="200",path="/api/users/.+"}
```

## Useful Queries

```promql
# Requests per second
rate(http_requests_total[5m])

# 95th percentile response time
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))

# Error rate
sum(rate(http_requests_total{status=~"5.."}[5m])) / sum(rate(http_requests_total[5m]))
```

## Docker Compose Example

```yaml
version: '3.8'
services:
  access-log-exporter:
    image: ghcr.io/jkroepke/access-log-exporter:latest
    ports:
      - "4040:4040"
      - "8514:8514/udp"

  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
```

## Troubleshooting

- **No metrics**: Check if access-log-exporter is receiving logs from your web server
- **High cardinality**: Use `simple_uri_upstream` preset for automatic path normalization
- **Performance**: Use 30-60 second scrape intervals for web metrics
