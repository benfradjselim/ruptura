#!/bin/sh
# Renders the console configuration from RUPTURA_BACKEND_URL, unless a
# default.conf is mounted, as the Helm chart does, then runs nginx.
set -eu
mkdir -p /tmp/nginx/conf.d
if [ -e /etc/nginx/conf.d/default.conf ]; then
  echo "ruptura: using the mounted /etc/nginx/conf.d/default.conf"
else
  backend=$(printf '%s' "${RUPTURA_BACKEND_URL}" | sed 's/[\\&|]/\\&/g')
  sed "s|\${RUPTURA_BACKEND_URL}|${backend}|g" /etc/nginx/ruptura/default.conf.template > /tmp/nginx/conf.d/default.conf
  echo "ruptura: console proxies /api/ to ${RUPTURA_BACKEND_URL}"
fi
# -e sends the startup log to stderr, before the configuration is read.
exec nginx -e stderr -c /etc/nginx/nginx.conf -g 'daemon off;'
