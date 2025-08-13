# Webserver Configuration

The access-log-exporter works with various web servers, including Nginx and Apache. This guide provides instructions for setting up the exporter with these servers.

By default, the exporter listens on port 8514/udp for incoming log messages in RFC3164 format.
We recommend using UDP for sending logs to the exporter.

access-log-exporter exclusively supports RFC3164 format for incoming logs.
Nginx and Apache2, among others, support this format.

The log format must use tab separation (`\t`) to avoid parsing issues. The exporter does not support JSON or other formats.

## nginx

Nginx can generate access logs compatible with access-log-exporter. This requires defining a custom log format in the Nginx configuration file.

Common locations for this file include `/etc/nginx/nginx.conf` or `/etc/nginx/conf.d/default.conf`, depending on the operating system.
You can also create an additional configuration file in the `/etc/nginx/conf.d/` directory.

access-log-exporter includes multiple presets. We recommend the `simple` or `simple_upstream` presets for most use cases. The `simple` preset logs the request method, status code, response time, and request/response sizes. The `simple_upstream` preset additionally logs upstream response times and status codes.

To configure Nginx, add the following lines to the configuration file.

```nginx
# Use only one of the presets below, depending on your needs.
# simple preset
log_format accesslog_exporter '$http_host\t$request_method\t$status\t$request_time\t$request_length\t$bytes_sent';
access_log syslog:server=127.0.0.1:8514 accesslog_exporter,nohostname;

# simple_upstream preset
log_format accesslog_exporter '$http_host\t$request_method\t$status\t$request_time\t$request_length\t$bytes_sent\t$upstream_addr\t$upstream_connect_time\t$upstream_header_time\t$upstream_response_time';
access_log syslog:server=127.0.0.1:8514 accesslog_exporter,nohostname;
```

References:
- [Nginx Documentation: Logging](https://nginx.org/en/docs/http/ngx_http_log_module.html)
- [Nginx Documentation: Syslog](https://nginx.org/en/docs/syslog.html)

### Exclude specific requests for access-log-exporter

To exclude specific requests from logging by access-log-exporter, use the `if` directive in Nginx. For example, to exclude requests to `/health` and `/metrics`, add the following lines:

```nginx
# Exclude specific requests from logging
map $request_uri $loggable {
    default 1;
    ~^/health 0;
    ~^/metrics 0;
}

# Use only one of the presets below, depending on your needs.
# simple preset with exclusion
log_format accesslog_exporter '$http_host\t$request_method\t$status\t$request_time\t$request_length\t$bytes_sent';
access_log syslog:server=127.0.0.1:8514 accesslog_exporter,nohostname if=$loggable;

# simple_upstream preset with exclusion
log_format accesslog_exporter '$http_host\t$request_method\t$status\t$request_time\t$request_length\t$bytes_sent\t$upstream_addr\t$upstream_connect_time\t$upstream_header_time\t$upstream_response_time';
access_log syslog:server=127.0.0.1:8514 accesslog_exporter,nohostname if=$loggable;
```

## Apache2

Apache can generate access logs compatible with access-log-exporter. This requires the `mod_log_config` module to define a custom log format. Recording incoming and outgoing request sizes requires the `mod_logio` module.

Add the following lines to the Apache configuration file. Common locations for this file include `/etc/apache2/apache2.conf` and `/etc/httpd/conf/httpd.conf`, depending on the operating system.

Adjust the binary path for the logger command if your system uses a different location.

```apache
# Configuration for the access-log-exporter
LogFormat "%v\t%m\t%>s\t%{ms}T\t%I\t%O" accesslog_exporter
CustomLog "|/usr/bin/logger --rfc3164 --server 127.0.0.1 --port 8514 --udp" accesslog_exporter
```

### Important Considerations

Apache does not natively log information about upstream servers. To track upstream response times or status codes, integrate additional modules or external tools.
