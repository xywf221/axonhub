# IP Access Control

AxonHub provides a global IP allowlist that restricts access to the entire instance.

## Overview

When enabled, only requests from IP addresses or CIDR ranges in the allowlist are accepted; all others are denied. This is useful when you want to lock down your AxonHub instance to only accept traffic from known networks (e.g., your corporate VPN or cloud VPC).

## Configuration

IP Access Control is configured via the AxonHub configuration file or environment variables:

```yaml
ip_access_control:
  enabled: true
  allowed_ips:
    - "10.0.0.0/8"
    - "192.168.1.0/24"
    - "203.0.113.10"
  redirect_url: ""  # Optional: redirect denied requests to this URL
```

## Behavior

- When **disabled** (default): all requests are allowed through.
- When **enabled**: requests from IPs matching any entry in `allowed_ips` are allowed; all others receive a 404 (or 302 redirect if `redirect_url` is set).
- The client IP is determined from the TCP connection's remote address.
