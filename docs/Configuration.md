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
    	path to one .yaml config file (env: CONFIG_CONFIG)
  --debug.enable
    	Enables go profiling endpoint. This should be never exposed. (env: CONFIG_DEBUG_ENABLE)
  --preset string
    	Preset configuration to use. Available presets: simple, simple_upstream, all. Custom presets can be defined via config file. Default is simple. (env: CONFIG_PRESET) (default "simple")
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
- **Container location:** `/config.yaml`
- **Manual installation:** `config.yaml` in the current directory
- **Custom location:** Use the `--config` flag to specify a different path

### Configuration File

A example configuration can be found [here](https://github.com/jkroepke/access-log-exporter/blob/main/packaging/etc/access-log-exporter/config.yaml).

## Presets

Presets define how incoming log messages transform into Prometheus metrics.
access-log-exporter includes three built-in presets and supports custom preset definitions.

### Built-in Presets

#### `simple` Preset

The `simple` preset provides basic HTTP metrics without upstream server information. Compatible with both Nginx and Apache2.

**Log format requirements:**
- **Nginx:** `'$http_host\t$request_method\t$status\t$request_time\t$request_length\t$bytes_sent'`
- **Apache2:** `"%v\t%m\t%>s\t%{ms}T\t%I\t%O"`

**Metrics generated:**
- `http_requests_total` - Counter of total HTTP requests
- `http_request_size_bytes` - Histogram of request sizes
- `http_response_size_bytes` - Histogram of response sizes
- `http_response_duration_seconds` - Histogram of response times

#### `simple_upstream` Preset

The `simple_upstream` preset extends the simple preset with upstream server metrics.
Only compatible with nginx, because apache2 does not support upstream metrics in the same way.

**Log format requirements:**
- **Nginx:** `'$http_host\t$request_method\t$status\t$request_time\t$request_length\t$bytes_sent\t$upstream_addr\t$upstream_connect_time\t$upstream_header_time\t$upstream_response_time'`

**Additional metrics:**
- `http_upstream_connect_duration_seconds` - Histogram of upstream connection times
- `http_upstream_header_duration_seconds` - Histogram of upstream header receive times
- `http_upstream_response_duration_seconds` - Histogram of upstream response times

#### `all` Preset

The `all` preset provides comprehensive metrics including user agent parsing,
SSL information, and upstream details with server labels.

**Log format requirements:**
- **Nginx:** `'$http_host\t$request_method\t$status\t$request_time\t$request_length\t$bytes_sent\t$upstream_addr\t$upstream_connect_time\t$upstream_header_time\t$upstream_response_time\t$upstream_cache_status\t$remote_user\t$https\t$server_protocol\t$http_user_agent'`

**Additional features:**
- User agent parsing
- SSL/TLS protocol information
- Upstream server labeling
- Cache status tracking
- Remote user identification

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
- name: "http_response_duration_seconds"
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
  - **`replacements`**: Array of regular expressions replacements for label values. Only the first matching replacement applies.

<details>
<summary>Understanding `replacements`</summary>

Replacements allow you to transform raw log field values into more meaningful or consistent label values
using regular expressions.
This helps reduce label cardinality and standardize values.

**Important behavior:**
- Replacements process in the order defined in the array
- Only the **first matching** replacement applies
- If no replacements match, the original value remains unchanged
- Empty matches can transform empty/null values into meaningful labels
- **Uses RE2 regular expression engine**: Does not support negative lookahead/lookbehind assertions

**Regular expression engine limitations:**

access-log-exporter uses Google's RE2 regular expression engine,
which is fast and safe but has some limitations compared to PCRE or Perl regex:

- **No negative lookahead**: `(?!pattern)` not supported
- **No negative lookbehind**: `(?<!pattern)` not supported
- **No backreferences**: `\1`, `\2` not supported
- **No recursion**: Recursive patterns not supported

For a complete list of supported syntax, see the [RE2 documentation](https://github.com/google/re2/wiki/Syntax).

**Common use cases and examples:**

```yaml
# Group HTTP status codes into classes
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

# Handle SSL/HTTPS values (from the built-in 'all' preset)
- name: "ssl"
  lineIndex: 12  # HTTPS field ($https in Nginx)
  replacements:
    - regexp: "^$"        # Empty value means no SSL
      replacement: "off"
    - regexp: "^on$"      # Explicit "on" value
      replacement: "on"
    # Any other value (like SSL protocol names) becomes "on"
    - regexp: ".*"
      replacement: "on"

# Simplify user agent strings to browser families
# Note: Order matters since we can't use negative lookahead
- name: "browser_family"
  lineIndex: 14  # User agent field
  replacements:
    # Chrome must come before Safari since Chrome contains "Safari"
    - regexp: ".*Chrome.*"
      replacement: "chrome"
    - regexp: ".*Firefox.*"
      replacement: "firefox"
    # This will match Safari that doesn't contain Chrome (due to ordering)
    - regexp: ".*Safari.*"
      replacement: "safari"
    - regexp: ".*[Bb]ot.*"
      replacement: "bot"
    - regexp: ".*curl.*"
      replacement: "curl"
    - regexp: ".*"
      replacement: "other"

# Group request methods
- name: "method_class"
  lineIndex: 1  # HTTP method field
  replacements:
    - regexp: "^(GET|HEAD|OPTIONS)$"
      replacement: "read"
    - regexp: "^(POST|PUT|PATCH)$"
      replacement: "write"
    - regexp: "^DELETE$"
      replacement: "delete"
    - regexp: ".*"
      replacement: "other"

# Convert upstream cache status to simplified values
- name: "cache_status"
  lineIndex: 10  # $upstream_cache_status field
  replacements:
    - regexp: "^(HIT|STALE)$"
      replacement: "hit"
    - regexp: "^(MISS|BYPASS|EXPIRED)$"
      replacement: "miss"
    - regexp: "^-$"           # No upstream cache
      replacement: "none"
    - regexp: ".*"
      replacement: "other"
```

**Example from the built-in `all` preset:**

The `all` preset uses this replacement pattern:
```yaml
- name: "ssl"
  lineIndex: 12
  replacements:
    - regexp: "^$"
      replacement: "off"
```

This transforms empty SSL values (when HTTPS is not used) into the explicit label value "off", making it clear when connections use HTTP vs HTTPS.

**Best practices:**
- Order replacements from most specific to most general
- Always include a catch-all pattern (`.*`) as the last replacement
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
- name: "http_response_duration_seconds"
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
- name: "http_upstream_response_duration_seconds"
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
