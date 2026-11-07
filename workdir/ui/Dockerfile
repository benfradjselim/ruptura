# Static files are pre-built by Vite into workdir/web/; the build context is ./workdir.
# nginx-unprivileged listens on 8080 and runs with any UID in group 0, as the
# OpenShift restricted-v2 SCC assigns.
FROM nginxinc/nginx-unprivileged:1.28-alpine

ARG VERSION=dev

LABEL name="ruptura-ui" \
      vendor="Ruptura" \
      version="${VERSION}" \
      release="1" \
      summary="Web console of the Ruptura engine" \
      description="Rupture horizon, forecast charts and call graph for the failures the Ruptura engine names." \
      maintainer="Selim Ben Fradj" \
      url="https://github.com/benfradjselim/ruptura" \
      io.k8s.display-name="Ruptura UI" \
      io.k8s.description="Web console of the Ruptura engine." \
      io.openshift.tags="ruptura,monitoring,observability" \
      org.opencontainers.image.source="https://github.com/benfradjselim/ruptura" \
      org.opencontainers.image.licenses="Apache-2.0"

# The engine Service the console proxies /api/ to. A mounted default.conf
# replaces this template, as the Helm chart does.
ENV RUPTURA_BACKEND_URL=http://ruptura:8080

COPY LICENSE /licenses/LICENSE
COPY ui/nginx/default.conf.template /etc/nginx/templates/default.conf.template
COPY web /usr/share/nginx/html

EXPOSE 8080
