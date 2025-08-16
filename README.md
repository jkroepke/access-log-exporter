[![CI](https://github.com/jkroepke/access-log-exporter/actions/workflows/ci.yaml/badge.svg?branch=main)](https://github.com/jkroepke/access-log-exporter/actions/workflows/ci.yaml)
[![GitHub license](https://img.shields.io/github/license/jkroepke/access-log-exporter)](https://github.com/jkroepke/access-log-exporter/blob/master/LICENSE.txt)
[![Current Release](https://img.shields.io/github/release/jkroepke/access-log-exporter.svg?logo=github)](https://github.com/jkroepke/access-log-exporter/releases/latest)
[![GitHub Repo stars](https://img.shields.io/github/stars/jkroepke/access-log-exporter?style=flat&logo=github)](https://github.com/jkroepke/access-log-exporter/stargazers)
[![GitHub all releases](https://img.shields.io/github/downloads/jkroepke/access-log-exporter/total?logo=github)](https://github.com/jkroepke/access-log-exporter/releases/latest)
[![Go Report Card](https://goreportcard.com/badge/github.com/jkroepke/access-log-exporter)](https://goreportcard.com/report/github.com/jkroepke/access-log-exporter)
[![codecov](https://codecov.io/gh/jkroepke/access-log-exporter/graph/badge.svg?token=TJRPHF5BVX)](https://codecov.io/gh/jkroepke/access-log-exporter)

# access-log-exporter

⭐ Don't forget to star this repository! ⭐

A Prometheus exporter that receives web server access logs via syslog and converts them into metrics.

access-log-exporter processes logs from multiple web servers and has undergone testing with Nginx and Apache HTTP Server 2.4. It supports flexible metric configuration through presets and provides comprehensive monitoring capabilities for web traffic analysis.

## Features

- **Multi-server support**: Works with Nginx and Apache HTTP Server,
- **Syslog protocol**: Receives logs via UDP/TCP syslog for real-time processing,
- **Flexible configuration**: Customizable presets for different monitoring needs,
- **Built-in presets**: Ready-to-use configurations for common scenarios,
- **Upstream metrics**: Support for Nginx upstream server monitoring,
- **Label transformation**: Regex-based label value normalization,
- **Mathematical operations**: Unit conversion for proper Prometheus base units,
- **High performance**: Configurable buffering and worker threads.

## Quick Start

### Installation

**Using package managers:**
```bash
# Debian/Ubuntu
curl -L https://raw.githubusercontent.com/jkroepke/access-log-exporter/refs/heads/main/packaging/apt/access-log-exporter.sources | sudo tee /etc/apt/sources.list.d/access-log-exporter.sources
sudo apt update && sudo apt install access-log-exporter

# Manual download
# Download the latest release from GitHub releases page
```

**Using Docker:**
```bash
docker run -p 4040:4040 -p 8514:8514/udp ghcr.io/jkroepke/access-log-exporter:latest
```

### Basic Configuration

**1. Configure your web server to send logs via syslog:**

For **Nginx**, add to your configuration:
```nginx
log_format accesslog_exporter '$http_host\t$request_method\t$status\t$request_time\t$request_length\t$bytes_sent';
access_log syslog:server=127.0.0.1:8514 accesslog_exporter,nohostname;
```

For **Apache2**, add to your configuration:
```apache
LogFormat "%v\t%m\t%>s\t%{ms}T\t%I\t%O" accesslog_exporter
CustomLog "|/usr/bin/logger --rfc3164 --server 127.0.0.1 --port 8514 --udp" accesslog_exporter
```

**2. Start access-log-exporter:**
```bash
access-log-exporter --preset simple
```

**3. Access metrics:**
```bash
curl http://localhost:4040/metrics
```

## Available Presets

- **`simple`**: Basic HTTP metrics (requests, response times, sizes) - compatible with both Nginx and Apache
- **`simple_upstream`**: Includes upstream server metrics - Nginx only
- **`all`**: Comprehensive metrics including user agent parsing, SSL info, and upstream details - Nginx only

## Configuration

access-log-exporter supports configuration via:
- Command-line flags
- Environment variables
- YAML configuration files

**Example command-line usage:**
```bash
# Use different preset
access-log-exporter --preset simple_upstream

# Custom syslog port
access-log-exporter --syslog.listen-address udp://0.0.0.0:9514

# Custom metrics port
access-log-exporter --web.listen-address :9090
```

**Example configuration file:**
```yaml
preset: "simple"
syslog:
  listenAddress: "udp://[::]:8514"
web:
  listenAddress: ":4040"
bufferSize: 1000
workerCount: 4
```

## Metrics Examples

With the `simple` preset, you get metrics like:

```prometheus
# HELP http_requests_total The total number of client requests
# TYPE http_requests_total counter
http_requests_total{host="example.com",method="GET",status="200"} 1234

# HELP http_request_duration_seconds The time spent on receiving and response the response to the client
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{host="example.com",method="GET",status="200",le="0.005"} 123
http_request_duration_seconds_bucket{host="example.com",method="GET",status="200",le="0.01"} 234
```

## Nginx Status Metrics

In addition to processing access logs, access-log-exporter can also collect Nginx server status metrics by scraping the `stub_status` module. This provides additional insights into your Nginx server's performance and connection handling.

### Enabling Nginx Status Collection

To enable Nginx status metrics collection, use the `--nginx.scrape-url` flag:

```bash
# HTTP endpoint
access-log-exporter --nginx.scrape-url http://127.0.0.1/stub_status

# Unix domain socket
access-log-exporter --nginx.scrape-url unix:///var/run/nginx-status.sock
```

### Nginx Configuration

First, enable the `stub_status` module in your Nginx configuration:

```nginx
server {
    listen 127.0.0.1:8080;
    server_name localhost;

    location /stub_status {
        stub_status on;
        access_log off;
        allow 127.0.0.1;
        deny all;
    }
}
```

**Important:** Ensure the stub_status endpoint is only accessible from localhost or trusted networks for security.

For detailed information about the metrics exposed and configuration options, see the [Nginx Status Metrics section](https://github.com/jkroepke/access-log-exporter/wiki/Configuration#nginx-status-metrics) in the Configuration Guide.

### Configuration File Example

You can also configure the Nginx scrape URL in your YAML configuration file:

```yaml
nginx:
  scrapeUri: "http://127.0.0.1:8080/stub_status"

preset: "simple"
syslog:
  listenAddress: "udp://[::]:8514"
web:
  listenAddress: ":4040"
```

## Documentation

For detailed documentation, please refer to:

- **[Installation Guide](https://github.com/jkroepke/access-log-exporter/wiki/Installation)**: Package installation, manual builds, Kubernetes deployment
- **[Configuration Guide](https://github.com/jkroepke/access-log-exporter/wiki/Configuration)**: Complete configuration reference and custom presets
- **[Webserver Setup](https://github.com/jkroepke/access-log-exporter/wiki/Webserver)**: Nginx and Apache configuration examples
- **[Wiki](https://github.com/jkroepke/access-log-exporter/wiki)**: Additional guides and examples

## Requirements

- Go 1.21+ (for building from source)
- Web server with syslog support (Nginx, Apache)
- Network connectivity between web server and access-log-exporter

## Contributing

Contributions welcome! Please read our [Code of Conduct](CODE_OF_CONDUCT.md) and submit pull requests to help improve the project.

## Related Projects

* [martin-helmich/prometheus-nginxlog-exporter](https://github.com/martin-helmich/prometheus-nginxlog-exporter).
* [ozonru/accesslog-exporter](https://github.com/ozonru/accesslog-exporter)

## Copyright and license

© 2025 Jan-Otto Kröpke (jkroepke)

Licensed under the [Apache License, Version 2.0](LICENSE.txt).

## Open Source Sponsors

Thanks to all sponsors!

## Acknowledgements

Thanks to JetBrains IDEs for their support.

<table>
  <thead>
    <tr>
      <th><a href="https://www.jetbrains.com/?from=jkroepke">JetBrains IDEs</a></th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td>
        <p align="center">
          <a href="https://www.jetbrains.com/?from=jkroepke">
            <picture>
              <source srcset="https://www.jetbrains.com/company/brand/img/logo_jb_dos_3.svg" media="(prefers-color-scheme: dark)">
              <img src="https://resources.jetbrains.com/storage/products/company/brand/logos/jetbrains.svg" style="height: 50px">
            </picture>
          </a>
        </p>
      </td>
    </tr>
  </tbody>
</table>
