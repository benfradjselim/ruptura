#!/bin/sh
set -e

RUPTURA_BACKEND_URL="${RUPTURA_BACKEND_URL:-http://ruptura:8080}"

# Substitute only RUPTURA_BACKEND_URL, leaving nginx's $uri / $host etc. intact.
envsubst '${RUPTURA_BACKEND_URL}' \
  < /tmp/nginx.conf.template \
  > /etc/nginx/conf.d/default.conf

exec nginx -g 'daemon off;'
