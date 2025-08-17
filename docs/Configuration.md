# Configuration

access-log-exporter supports configuration through command-line flags,
environment variables, and YAML configuration files.
This document explains all available configuration options and how to use them effectively.

## Configuration Priority

Configuration options follow this priority order (highest to lowest):
1. Command-line flags
2. Environment variables
3. Configuration file
4. Default values

## Command-Line Flags

You can configure access-log-exporter using command-line flags.
Each flag also supports environment variable configuration using the format shown in parentheses.

```
Documentation available at https://github.com/jkroepke/access-log-exporter/wiki

Usage of access-log-exporter:

  --buffer-size uint
    	Size of the buffer for syslog messages. Default is 1000. Set to 0 to disable buffering. (env: CONFIG_BUFFER__SIZE) (default 1000)
  --config string
    	path to one .yaml config file (env: CONFIG_FILE) (default "config.yaml")
  --debug.enable
    	Enables go profiling endpoint. This should be never exposed. (env: CONFIG_DEBUG_ENABLE)
  --nginx.scrape-url value
    	A URI or unix domain socket path for scraping NGINX metrics. For NGINX, the stub_status page must be available through the URI. Examples: http://127.0.0.1/stub_status or `unix:///var/run/nginx-status.sock` (env: CONFIG_NGINX_SCRAPE__URL)
  --preset string
    	Preset configuration to use. Available presets: simple, simple_upstream, simple_uri_upstream. Custom presets can be defined via config file. Default is simple. (env: CONFIG_PRESET) (default "simple")
  --syslog.listen-address string
    	Addresses on which to expose syslog. Examples: udp://0.0.0.0:8514, tcp://0.0.0.0:8514, unix:///path/to/socket. (env: CONFIG_SYSLOG_LISTEN__ADDRESS) (default "udp://[::]:8514")
  --verify-config
    	Enable this flag to check config file loads, then exit (env: CONFIG_VERIFY__CONFIG)
  --version
    	show version
  --web.listen-address :4041
    	Addresses on which to expose metrics. Examples: :4041 or `[::1]:4041` for http (env: CONFIG_WEB_LISTEN__ADDRESS) (default ":4040")
  --worker int
    	Number of workers to process syslog messages. 0 or below means number of available CPU cores. (env: CONFIG_WORKER)
```

## Configuration File

access-log-exporter uses YAML configuration files for advanced configuration options. The configuration file allows you to define custom presets, detailed metric configurations, and logging settings.

### Default Configuration File Location

The default configuration file location depends on your installation method:

- **Package installation:** `/etc/access-log-exporter/config.yaml`
- **Container location:** `/var/run/ko/config.yaml`
- **Manual installation:** `config.yaml` in the current directory
- **Custom location:** Use the `--config` flag to specify a different path

### Configuration File

A example configuration can be found [here](https://github.com/jkroepke/access-log-exporter/blob/main/packaging/etc/access-log-exporter/config.yaml).

## Nginx Status Metrics

access-log-exporter can collect Nginx server status metrics in addition to processing access logs. This feature uses Nginx's `stub_status` module to provide insights into server performance and connection handling.

### Configuration

To enable Nginx status metrics collection, configure the scrape URL using either:

**Command-line flag:**
```bash
access-log-exporter --nginx.scrape-url http://127.0.0.1:8080/stub_status
```

**Environment variable:**
```bash
export CONFIG_NGINX_SCRAPE__URL="http://127.0.0.1:8080/stub_status"
access-log-exporter
```

**YAML configuration file:**
```yaml
nginx:
  scrapeUri: "http://127.0.0.1:8080/stub_status"
```

### Supported URL Schemes

The nginx.scrape-url supports these URL schemes:

- **HTTP endpoints:** `http://127.0.0.1:8080/stub_status`
- **HTTPS endpoints:** `https://nginx.example.com/stub_status`

### Nginx Configuration Requirements

To use this feature, you must enable nginx's `stub_status` module:

```nginx
server {
    listen 127.0.0.1:8080;
    server_name localhost;

    location /stub_status {
        stub_status on;
        access_log off;
        allow 127.0.0.1;
        deny all;
        server_tokens on;  # expose nginx version
    }
}
```

**Security considerations:**
- Restrict access to localhost or trusted networks only
- Use `allow`/`deny` directives to control access
- Disable access logging for the status endpoint

### Metrics Exposed

When enabled, the following Nginx-specific metrics are collected and exposed:

| Metric Name                        | Type    | Description                                                        |
|------------------------------------|---------|--------------------------------------------------------------------|
| `nginx_up`                         | Gauge   | Whether the NGINX server is up (1) or down (0)                     |
| `nginx_connections_accepted_total` | Counter | Total number of accepted client connections                        |
| `nginx_connections_active`         | Gauge   | Current number of active client connections                        |
| `nginx_connections_handled_total`  | Counter | Total number of handled client connections                         |
| `nginx_connections_reading`        | Gauge   | Connections where NGINX is reading the request header              |
| `nginx_connections_writing`        | Gauge   | Connections where NGINX is writing the response back to the client |
| `nginx_connections_waiting`        | Gauge   | Idle client connections (keep-alive)                               |

### Example Output

```prometheus
# HELP nginx_up Whether the NGINX server is up (1) or down (0)
# TYPE nginx_up gauge
nginx_up{version="1.28.0"} 1

# HELP nginx_connections_accepted_total Accepted client connections
# TYPE nginx_connections_accepted_total counter
nginx_connections_accepted_total 15234

# HELP nginx_connections_active Active client connections
# TYPE nginx_connections_active gauge
nginx_connections_active 3

# HELP nginx_connections_handled_total Handled client connections
# TYPE nginx_connections_handled_total counter
nginx_connections_handled_total 15234

# HELP nginx_connections_reading Connections where NGINX is reading the request header
# TYPE nginx_connections_reading gauge
nginx_connections_reading 0

# HELP nginx_connections_writing Connections where NGINX is writing the response back to the client
# TYPE nginx_connections_writing gauge
nginx_connections_writing 1

# HELP nginx_connections_waiting Idle client connections
# TYPE nginx_connections_waiting gauge
nginx_connections_waiting 2
```

### Error Handling

If the Nginx status endpoint is unreachable or returns invalid data:
- The `nginx_up` metric will be set to `0`
- Other Nginx metrics will not be updated
- Error details are logged for troubleshooting
- Access log processing continues normally
- If the `version` label is set to `N/A`, check if `server_tokens on` is set within location block.

This allows you to monitor both the availability of your Nginx server and the health of the metrics collection process.

## Presets

Presets define how incoming log messages transform into Prometheus metrics.
access-log-exporter includes four built-in presets and supports custom preset definitions.

### Built-in Presets

#### `simple` Preset

The `simple` preset provides basic HTTP metrics without upstream server information. Compatible with both Nginx and Apache2.

**Log format requirements:**
- **Nginx:** `'$http_host\t$request_method\t$status\t$request_completion\t$request_time\t$request_length\t$bytes_sent'`
- **Apache2:** `"%v\t%m\t%>s\tOK\t%{ms}T\t%I\t%O"`

**Metrics generated:**
- `http_requests_total` - Counter of total HTTP requests
- `http_request_size_bytes` - Histogram of request sizes
- `http_response_size_bytes` - Histogram of response sizes
- `http_request_duration_seconds` - Histogram of response times

#### `simple_upstream` Preset

The `simple_upstream` preset extends the simple preset with upstream server metrics.
Only compatible with nginx, because apache2 does not support upstream metrics in the same way.

**Log format requirements:**
- **Nginx:** `'$http_host\t$request_method\t$status\t$request_completion\t$request_time\t$request_length\t$bytes_sent\t$upstream_addr\t$upstream_connect_time\t$upstream_header_time\t$upstream_response_time'`

**Additional metrics:**
- `http_upstream_connect_duration_seconds` - Histogram of upstream connection times
- `http_upstream_header_duration_seconds` - Histogram of upstream header receive times
- `http_upstream_request_duration_seconds` - Histogram of upstream response times

#### `simple_uri_upstream` Preset

The `simple_uri_upstream` preset extends the simple_upstream preset with request URI tracking.
Only compatible with nginx, because apache2 does not support upstream metrics in the same way.

**Log format requirements:**
- **Nginx:** `'$http_host\t$request_method\t$status\t$request_completion\t$request_time\t$request_length\t$bytes_sent\t$upstream_addr\t$upstream_connect_time\t$upstream_header_time\t$upstream_response_time\t$request_uri'`

**Additional features:**
- All metrics from `simple_upstream` preset
- Request URI tracking with automatic path normalization
- URI paths are normalized to reduce cardinality (e.g., `/api/users/123/profile` becomes `/api/users/.+`)

**Additional labels:**
- `request_uri` - Added to all metrics with path normalization

### Custom Presets

You can define custom presets in the configuration file under the `presets` section.
Each preset contains a list of metrics with their configuration.

#### Metric Types

access-log-exporter supports these Prometheus metric types:

- **`counter`**: Monotonically increasing values (e.g., request counts)
- **`histogram`**: Distribution of values with configurable buckets (e.g., response times)

#### Metric Configuration Options

Each metric supports these configuration options:

##### Basic Options
- **`name`**: Metric name (must follow Prometheus naming conventions)
- **`type`**: Metric type (`counter` or `histogram`)
- **`help`**: Description of what the metric measures
- **`valueIndex`**: Specifies, which field from the tab-separated log line contains the numeric value for this metric. Only required for histogram metrics. Fields start counting from 0 (zero-based indexing).

<details>
<summary>Understanding `valueIndex` with examples</summary>

When a log line arrives, access-log-exporter splits it by tab characters (`\t`) into numbered fields:

**Example Nginx log line:**
```
example.com\tGET\t200\t0.123\t456\t1024
```

This creates these indexed fields:
- Field 0: `example.com` (host)
- Field 1: `GET` (method)
- Field 2: `200` (status)
- Field 3: `0.123` (response time in seconds)
- Field 4: `456` (request size in bytes)
- Field 5: `1024` (response size in bytes)

**Example metric configurations:**

```yaml
# Response time histogram - uses field 3 (0.123)
- name: "http_request_duration_seconds"
  type: "histogram"
  valueIndex: 3  # Points to the response time field
  help: "Response duration in seconds"

# Request size histogram - uses field 4 (456)
- name: "http_request_size_bytes"
  type: "histogram"
  valueIndex: 4  # Points to the request size field
  help: "Request size in bytes"

# Counter metrics don't need valueIndex - they just count occurrences
- name: "http_requests_total"
  type: "counter"
  help: "Total number of requests"
  # No valueIndex needed for counters
```

**Important notes:**
- Counter metrics can either count log line occurrences (without `valueIndex`) or sum numeric values from a specific field (with `valueIndex`)
- Histogram metrics require `valueIndex` to know which field contains the measurable value
- Field indexing starts at 0, not 1
- The field must contain a valid numeric value when using `valueIndex`

**Counter metric examples:**

```yaml
# Counter that counts occurrences (no valueIndex)
- name: "http_requests_total"
  type: "counter"
  help: "Total number of requests"
  # No valueIndex - counts each log line

# Counter that sums response sizes (with valueIndex)
- name: "http_response_bytes_total"
  type: "counter"
  help: "Total bytes sent to clients"
  valueIndex: 5  # Sums the response size field

# Histogram that tracks response size distribution
- name: "http_response_size_bytes"
  type: "histogram"
  help: "Distribution of response sizes"
  valueIndex: 5  # Same field, but creates buckets
  buckets: [100, 1000, 10000, 100000]
```

</details>

##### Label Configuration
- **`labels`**: Array of label definitions
  - **`name`**: Label name
  - **`lineIndex`**: Index of the log field for this label
  - **`userAgent`**: Enable user agent parsing (boolean)
  - **`replacements`**: Array of string or regular expression replacements for label values. Only the first matching replacement applies.
    - **`string`**: Exact string to match and replace
    - **`regexp`**: Regular expression pattern to match
    - **`replacement`**: Value to replace the matched string/pattern with. If `regexp` is set, capture groups can be used in the replacement string using `$1`, `$2`, etc.

<details>
<summary>Understanding `replacements`</summary>

Replacements allow you to transform raw log field values into more meaningful or consistent label values
using either exact string matching or regular expressions.
This helps reduce label cardinality and standardize values.

**Replacement Types:**

access-log-exporter supports two types of replacements:

1. **String replacements**: Exact string matching for simple transformations
2. **Regular expression replacements**: Pattern-based matching for complex transformations

**Important behavior:**
- Replacements process in the order defined in the array
- Only the **first matching** replacement applies
- If no replacements match, the original value remains unchanged
- Empty matches can transform empty/null values into meaningful labels
- **Uses RE2 regular expression engine**: Does not support negative lookahead/lookbehind assertions

**String Replacement Examples:**

String replacements are perfect for simple, exact value transformations:

```yaml
# Simple string-based replacements for request completion status
- name: "completion_status"
  lineIndex: 3  # $request_completion field
  replacements:
    - string: "OK"        # Exact match for "OK"
      replacement: "1"
    - string: ""          # Exact match for empty string
      replacement: "0"

# Transform specific method names
- name: "method_normalized"
  lineIndex: 1  # HTTP method field
  replacements:
    - string: "GET"
      replacement: "read"
    - string: "POST"
      replacement: "write"
    - string: "PUT"
      replacement: "write"
    - string: "DELETE"
      replacement: "delete"
```

**Regular Expression Examples:**

Regular expressions provide powerful pattern-based matching for complex transformations:

```yaml
# Group HTTP status codes into classes using regex
- name: "status_class"
  lineIndex: 2  # HTTP status field
  replacements:
    - regexp: "^2..$"
      replacement: "2xx"
    - regexp: "^3..$"
      replacement: "3xx"
    - regexp: "^4..$"
      replacement: "4xx"
    - regexp: "^5..$"
      replacement: "5xx"
    - regexp: ".*"  # Catch-all for any other values
      replacement: "other"

# Handle SSL/HTTPS values with regex patterns
- name: "ssl"
  lineIndex: 12  # HTTPS field ($https in Nginx)
  replacements:
    - regexp: "^$"        # Empty value means no SSL
      replacement: "off"
    - regexp: "^on$"      # Explicit "on" value
      replacement: "on"
    # Any other value (like SSL protocol names) becomes "on"
```

**When to use which type:**

- **Use `string` replacements** for:
  - Exact value matching (like "OK" → "1")
  - Simple transformations
  - Better performance with known fixed values
  - Empty string handling

- **Use `regexp` replacements** for:
  - Pattern-based matching (like status code ranges)
  - Complex string transformations
  - Wildcard matching
  - Path normalization

**Mixed usage example:**

You can mix both types in the same replacement array:

```yaml
- name: "normalized_value"
  lineIndex: 5
  replacements:
    # Handle specific known values with string matching (faster)
    - string: "OK"
      replacement: "success"
    - string: "FAILED"
      replacement: "error"
    # Handle patterns with regex (more flexible)
    - regexp: "^ERROR_.*"
      replacement: "error"
    - regexp: "^WARN_.*"
      replacement: "warning"
    # Catch-all
    - regexp: ".*"
      replacement: "unknown"
```

**Regular expression engine limitations:**

access-log-exporter uses Google's RE2 regular expression engine,
which is fast and safe but has some limitations compared to PCRE or Perl regular expression engines:

- **No lookahead assertions**: `(?=pattern)` not supported
- **No lookbehind assertions**: `(?<=pattern)` not supported
- **No named capture groups**: `(?P<name>pattern)` not supported
- **No non-capturing groups**: `(?:pattern)` not supported
- **No negative lookahead**: `(?!pattern)` not supported
- **No negative lookbehind**: `(?<!pattern)` not supported
- **No backreferences**: `\1`, `\2` not supported
- **No recursion**: Recursive patterns not supported

For a complete list of supported syntax, see the [RE2 documentation](https://github.com/google/re2/wiki/Syntax).

**Best practices:**
- Use `string` replacements for exact matches when possible (better performance)
- Use `regexp` replacements for pattern matching and complex transformations
- Order replacements from most specific to most general
- Always include a catch-all pattern (`.*`) as the last replacement when using regex
- Use replacements to reduce label cardinality (group similar values)
- Test regular expression patterns to ensure they match expected log values
- Consider the performance impact of complex regular expression patterns on high-traffic logs
- **Remember RE2 limitations** - use ordering instead of negative lookahead/lookbehind

</details>

##### Histogram Options
- **`buckets`**: Array of bucket boundaries for histogram metrics

**Recommended bucket values:**

For **size-related metrics** (request/response sizes in bytes):
```yaml
buckets: [10, 20, 30, 40, 50, 60, 70, 80, 90, 100]
```

For **time-related metrics** (response times, connection times in seconds):
```yaml
buckets: [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10]
```

These values provide good coverage for typical web traffic patterns.
You can customize them based on your specific application's characteristics.

##### Mathematical Operations
- **`math`**: Mathematical transformations for converting values to proper base units
  - **`enabled`**: Enable mathematical operations
  - **`mul`**: Multiply value by this factor
  - **`div`**: Divide value by this factor

<details>
<summary>Why mathematical operations matter</summary>

Prometheus metrics should always use base units for consistency and proper alerting. Web servers often log values in different units than what Prometheus expects:

- **Time metrics**: Should use seconds, but servers often log in milliseconds
- **Size metrics**: Should use bytes, but servers might log in kilobytes or other units

**Common transformations:**

```yaml
# Convert milliseconds to seconds (divide by 1000)
- name: "http_request_duration_seconds"
  type: "histogram"
  valueIndex: 3  # Field contains time in milliseconds
  math:
    enabled: true
    div: 1000  # Convert ms to seconds
  help: "Response duration in seconds"

# Convert microseconds to seconds (divide by 1,000,000)
- name: "http_upstream_connect_duration_seconds"
  type: "histogram"
  valueIndex: 7  # Field contains time in microseconds
  math:
    enabled: true
    div: 1000000  # Convert μs to seconds
  help: "Upstream connection time in seconds"

# Convert kilobytes to bytes (multiply by 1024)
- name: "http_response_size_bytes"
  type: "histogram"
  valueIndex: 5  # Field contains size in KB
  math:
    enabled: true
    mul: 1024  # Convert KB to bytes
  help: "Response size in bytes"
```

**Real-world example from the built-in presets:**

The `simple` preset uses `div: 1000`
because Nginx's `$request_time` variable outputs time in seconds with millisecond precision
(e.g., "0.123"),
but when logged, it often gets converted to milliseconds.
The division ensures the final metric uses seconds as the base unit.

</details>

##### Upstream Configuration
- **`upstream`**: Upstream server handling for Nginx upstream variables
  - **`enabled`**: Enable upstream processing
  - **`addrLineIndex`**: Log field index containing upstream address
  - **`label`**: Include upstream address as a label
  - **`excludes`**: Array of upstream addresses to exclude

<details>
<summary>Why upstream configuration is necessary</summary>

Nginx upstream variables have a special multi-value format that requires special handling. When Nginx processes requests through upstream servers, it can contact multiple servers during a single request, and the upstream variables contain comma and colon-separated values.

According to the [Nginx upstream module documentation](https://nginx.org/en/docs/http/ngx_http_upstream_module.html#variables), upstream variables like `$upstream_addr` can contain:

- **Single server**: `192.168.1.1:80`
- **Multiple servers**: `192.168.1.1:80, 192.168.1.2:80, unix:/tmp/sock`
- **Server group redirects**: `192.168.1.1:80, 192.168.1.2:80 : 192.168.10.1:80, 192.168.10.2:80`

Other upstream variables follow the same pattern:
- `$upstream_bytes_received` - bytes received from upstream servers
- `$upstream_bytes_sent` - bytes sent to upstream servers
- `$upstream_connect_time` - connection establishment times
- `$upstream_header_time` - header receive times
- `$upstream_response_time` - response times

**How upstream processing works:**

When `upstream.enabled: true`, access-log-exporter:

1. **Parses multi-value fields**: Splits comma and colon-separated values
2. **Creates separate metrics**: Generates one metric entry per upstream server
3. **Handles exclusions**: Skips upstream addresses listed in `excludes` array
4. **Adds labels**: Optionally includes upstream address as a metric label when `label: true`

**Configuration examples:**

```yaml
# Basic upstream processing without labels
- name: "http_upstream_request_duration_seconds"
  type: "histogram"
  valueIndex: 9  # $upstream_response_time field
  upstream:
    enabled: true
    addrLineIndex: 6  # $upstream_addr field
    # No label means upstream address won't be included as a metric label

# Upstream processing with server labels
- name: "http_upstream_connect_duration_seconds"
  type: "histogram"
  valueIndex: 7  # $upstream_connect_time field
  upstream:
    enabled: true
    addrLineIndex: 6  # $upstream_addr field
    label: true  # Include upstream address as "upstream" label
    excludes: ["unix:/tmp/sock"]  # Exclude Unix sockets

# Log line example with multiple upstream servers:
# example.com  GET  200  0.123  456  1024  192.168.1.1:80,192.168.1.2:80  0.050,0.055  0.100,0.110  0.120,0.125
#
# This creates two separate metric entries:
# 1. upstream="192.168.1.1:80" with connect_time=0.050, header_time=0.100, response_time=0.120
# 2. upstream="192.168.1.2:80" with connect_time=0.055, header_time=0.110, response_time=0.125
```

**Important notes:**
- Upstream configuration only applies to metrics with upstream-related `valueIndex` fields
- The `addrLineIndex` must point to the field containing `$upstream_addr` or equivalent
- Values in both the address field and value field must have matching comma/colon positions
- Use `excludes` to filter out specific upstream addresses (like Unix sockets or internal addresses)

</details>
