#!/bin/sh
# Prepares the writable paths nginx needs and runs it. The API key never
# reaches this container: the proxy helper holds it.
set -eu
mkdir -p /tmp/nginx
echo "ruptura: console plugin serving on 9443, engine reads through the proxy helper"
# -e sends the startup log to stderr, before the configuration is read.
exec nginx -e stderr -c /etc/nginx/nginx.conf -g 'daemon off;'
