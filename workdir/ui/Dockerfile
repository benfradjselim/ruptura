# The UI is built by Vite in the first stage; the build context is ./workdir.
# The UBI nginx listens on 8080 and runs with any UID, as the OpenShift
# restricted-v2 SCC assigns.
FROM --platform=$BUILDPLATFORM docker.io/library/node:22-alpine AS build
WORKDIR /src/ui
COPY ui/package.json ui/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY ui/ ./
RUN npm run build

FROM registry.access.redhat.com/ubi9/nginx-124

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

# The engine Service the console proxies /api/ to. A default.conf mounted
# read-only, as the Helm chart does, is used as is.
ENV RUPTURA_BACKEND_URL=http://ruptura:8080

USER 0
COPY LICENSE /licenses/LICENSE
COPY ui/nginx/nginx.conf /etc/nginx/nginx.conf
COPY ui/nginx/default.conf.template /etc/nginx/ruptura/default.conf.template
COPY --chmod=0755 ui/nginx/entrypoint.sh /usr/local/bin/ruptura-ui
COPY --from=build --chown=1001:0 /src/web /opt/app-root/src
USER 1001

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/ruptura-ui"]
