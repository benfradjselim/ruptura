#!/bin/sh
# Renders the proxy configuration from the engine URL and the API key file,
# then runs nginx. The key is read from the mounted Secret at start; a rotated
# key takes effect when the pod restarts. The key is never logged.
set -eu

backend="${RUPTURA_BACKEND_URL:-http://ruptura:8080}"
key_file="${RUPTURA_API_KEY_FILE:-/var/run/ruptura/api-key/api-key}"

if ! printf '%s' "$backend" | grep -Eq '^https?://[A-Za-z0-9.-]+(:[0-9]+)?$'; then
  echo "ruptura: RUPTURA_BACKEND_URL must be http(s)://host[:port], got $backend" >&2
  exit 1
fi
if [ ! -r "$key_file" ]; then
  echo "ruptura: API key file $key_file is not readable" >&2
  exit 1
fi
key="$(tr -d '\r\n' < "$key_file")"
if ! printf '%s' "$key" | grep -Eq '^[A-Za-z0-9._~+/=-]+$'; then
  echo "ruptura: API key in $key_file has characters that cannot be sent in a header" >&2
  exit 1
fi

umask 077
mkdir -p /tmp/ruptura /tmp/nginx
cat > /tmp/ruptura/proxy.conf <<EOF
limit_except GET { deny all; }
proxy_pass ${backend};
proxy_http_version 1.1;
proxy_set_header Authorization "Bearer ${key}";
proxy_set_header Cookie "";
proxy_read_timeout 30s;
EOF

echo "ruptura: console plugin proxies the engine read endpoints to ${backend}"
exec nginx -c /etc/nginx/nginx.conf -g 'daemon off;'
