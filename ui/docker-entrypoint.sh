#!/bin/sh
set -e

RUPTURA_BACKEND_URL="${RUPTURA_BACKEND_URL:-http://ruptura}"
RUPTURA_API_KEY="${RUPTURA_API_KEY:-}"

# Substitute only these two vars, leaving nginx's $uri / $host etc. intact.
envsubst '${RUPTURA_BACKEND_URL},${RUPTURA_API_KEY}' \
  < /tmp/nginx.conf.template \
  > /etc/nginx/conf.d/default.conf

exec nginx -g 'daemon off;'
