# Developer Guide

This document provides a technical overview of the project and highlights the most important packages and concepts.

## Overview

`access-log-exporter` is a high-performance Go application that acts as a Prometheus exporter for access logs. It receives log messages via syslog protocol, parses them according to configurable rules, and exposes the extracted metrics in Prometheus format.

### Key Features

- **High-throughput processing**: Concurrent worker architecture for processing thousands of log lines per second
- **Flexible configuration**: YAML-based configuration with support for multiple metric presets
- **Multiple metric types**: Support for Prometheus counters, gauges, and histograms
- **Label processing**: Advanced label extraction with regex replacements and user agent parsing
- **Upstream support**: Special handling for load balancer upstream servers
- **Memory efficient**: Uses sync.Pool for object reuse to minimize garbage collection pressure
- **Thread-safe**: Designed for concurrent access across multiple goroutines

## Architecture

The application follows a pipeline architecture with the following main components:

```
Syslog Server → Message Buffer → Worker Pool → Metric Processing → Prometheus Export
```

### Core Components

1. **Syslog Server** (`internal/syslog`): Receives and parses syslog messages
2. **Collector** (`internal/collector`): Manages worker pool and coordinates metric processing
3. **Metric Engine** (`internal/metric`): Processes log lines and updates Prometheus metrics
4. **Configuration** (`internal/config`): Handles YAML configuration and validation
5. **HTTP Server**: Exposes `/metrics` endpoint for Prometheus scraping

## How It Works

### 1. Startup and Initialization

The application starts by:

1. **Configuration Loading**: Reads YAML configuration file or environment variables
2. **Preset Selection**: Chooses the active metric preset from configuration
3. **Syslog Server Setup**: Creates UDP or Unix socket listener for syslog messages
4. **Metric Initialization**: Creates Prometheus metric collectors based on configuration
5. **Worker Pool**: Spawns concurrent workers (defaults to number of CPU cores)
6. **HTTP Server**: Starts web server for `/metrics` and `/health` endpoints

### 2. Message Flow

#### Syslog Reception
```go
// Syslog server receives messages on UDP/Unix socket
// Strips syslog headers and extracts the actual log message
// Sends cleaned message to buffered channel
```

The syslog component:
- Listens on UDP or Unix domain sockets
- Parses syslog format messages (e.g., `<34>Oct 11 22:14:15 nginx: actual_log_message`)
- Extracts the actual log message after the third colon
- Uses a buffer pool to minimize memory allocations

#### Concurrent Processing
```go
// Multiple worker goroutines process messages concurrently
func (c *Collector) lineHandlerWorker(ctx context.Context, logger *slog.Logger, messageCh <-chan string) {
    for msg := range messageCh {
        // Split message into fields (tab-separated)
        line := strings.Split(msg, "\t")

        // Process each configured metric
        for _, metric := range c.metrics {
            metric.Parse(line)
        }
    }
}
```

Workers operate independently and process messages from a shared channel, providing high throughput through parallel processing.

#### Metric Processing
```go
// Each metric processes the log line according to its configuration
func (m *Metric) Parse(line []string) error {
    // 1. Validate line format and extract value
    // 2. Get labels map from sync.Pool (thread-safe reuse)
    // 3. Process each configured label
    // 4. Apply transformations (user agent parsing, regex replacements)
    // 5. Set metric value (counter, gauge, or histogram)
    // 6. Return labels map to pool
}
```

### 3. Configuration System

The configuration system supports:

#### Presets
```yaml
presets:
  nginx:
    metrics:
      - name: http_requests_total
        type: counter
        help: "Total HTTP requests"
        labels:
          - name: host
            lineIndex: 0
          - name: method
            lineIndex: 1
```

#### Metric Types
- **Counter**: Monotonically increasing values (request counts, error counts)
- **Gauge**: Values that can go up and down (response times, queue sizes)
- **Histogram**: Distribution of values (response time distributions)

#### Advanced Features
- **Math transformations**: Apply multiplication/division to metric values
- **Label replacements**: Use regex to transform label values
- **User agent parsing**: Extract browser family from user agent strings
- **Upstream handling**: Special support for load balancer upstream servers

### 4. Performance Optimizations

#### Memory Management
- **sync.Pool**: Reuses `prometheus.Labels` maps across goroutines to reduce allocations
- **Buffer pooling**: Syslog server reuses byte buffers for reading messages
- **Pre-sized allocations**: Maps and slices are allocated with known capacity

#### Concurrency
- **Worker pool**: Parallel processing across multiple CPU cores
- **Thread-safe design**: All components designed for concurrent access
- **Channel-based communication**: Non-blocking message passing between components

#### Parsing Optimizations
- **Bounds check elimination**: Uses Go compiler hints to eliminate array bounds checks
- **Efficient string operations**: Uses `strings.IndexByte` for fast comma parsing
- **Regex optimization**: Only calls regex replacement when match is found

### 5. Key Packages

#### `internal/metric`
Core metric processing engine:
- `Parse()`: Main entry point for processing log lines
- `setMetric()`: Handles value parsing and Prometheus metric updates
- `labelValueReplacements()`: Applies regex transformations to labels
- Thread-safe design using sync.Pool for label map reuse

#### `internal/collector`
Manages the worker pool and coordinates metric processing:
- `lineHandlerWorkers()`: Creates concurrent worker goroutines
- `lineHandlerWorker()`: Individual worker that processes messages
- Implements Prometheus collector interface

#### `internal/syslog`
Handles syslog protocol reception:
- Supports UDP and Unix domain sockets
- Parses syslog format and extracts log messages
- Uses buffer pooling for high-performance message processing

#### `internal/config`
Configuration management and validation:
- YAML/JSON configuration parsing
- Environment variable support
- Configuration validation and defaults
- Support for multiple metric presets

### 6. Monitoring and Observability

The exporter includes built-in metrics:
- `log_parse_errors_total`: Counter of parsing errors
- `log_last_received_timestamp_seconds`: Timestamp of last received message
- Standard Go runtime metrics (memory, GC, goroutines)
- Optional nginx stub_status metrics

### 7. Testing and Benchmarking

The project includes comprehensive benchmarks:
- `BenchmarkMetricParseSimple`: Tests basic metric parsing performance
- `BenchmarkMetricParseUserAgent`: Tests user agent parsing overhead
- `BenchmarkMetricParseUpstream`: Tests upstream processing performance

Performance targets:
- Zero allocations in the hot path for simple metrics
- Sub-microsecond processing time per log line
- Scales linearly with number of CPU cores

## Development Workflow

1. **Code formatting**: Run `make fmt` to format Go code
2. **Linting**: Run `make lint` to check code quality
3. **Testing**: Run `make test` to execute test suite
4. **Benchmarking**: Use `go test -bench=.` to measure performance

The application is designed for high-throughput log processing environments where performance and memory efficiency are critical requirements.
