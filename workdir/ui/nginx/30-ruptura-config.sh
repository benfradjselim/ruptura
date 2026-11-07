#!/bin/sh
# Renders the console configuration from RUPTURA_BACKEND_URL, unless a
# default.conf is mounted read-only, as the Helm chart does.
set -eu
out=/etc/nginx/conf.d/default.conf
if [ -w "$out" ] || { [ ! -e "$out" ] && [ -w "$(dirname "$out")" ]; }; then
  envsubst '${RUPTURA_BACKEND_URL}' < /etc/nginx/ruptura/default.conf.template > "$out"
  echo "ruptura: console proxies /api/ to ${RUPTURA_BACKEND_URL}"
else
  echo "ruptura: using the mounted $out"
fi
