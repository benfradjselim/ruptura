# The plugin assets are built once on the build platform; only static files
# and the nginx configuration go into the image, so a multi-arch build needs
# no emulated npm. The UBI nginx runs as any UID, as the OpenShift
# restricted-v2 SCC assigns, and listens on 9443 with the serving certificate.
FROM --platform=$BUILDPLATFORM docker.io/library/node:22-alpine AS build
WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY . .
RUN npm run build

FROM registry.access.redhat.com/ubi9/nginx-124

ARG VERSION=dev

LABEL name="ruptura-console-plugin" \
      vendor="Ruptura" \
      version="${VERSION}" \
      release="1" \
      summary="OpenShift console plugin of the Ruptura engine" \
      description="Shows the failures the Ruptura engine names in the OpenShift console: Observe > Ruptures and a Ruptura tab on workloads." \
      maintainer="Selim Ben Fradj" \
      url="https://github.com/benfradjselim/ruptura" \
      io.k8s.display-name="Ruptura console plugin" \
      io.k8s.description="OpenShift console plugin of the Ruptura engine." \
      io.openshift.tags="ruptura,monitoring,observability,console-plugin" \
      org.opencontainers.image.source="https://github.com/benfradjselim/ruptura" \
      org.opencontainers.image.licenses="Apache-2.0"

USER 0
COPY LICENSE /licenses/LICENSE
COPY nginx/nginx.conf /etc/nginx/nginx.conf
COPY --chmod=0755 nginx/entrypoint.sh /usr/local/bin/ruptura-console-plugin
COPY --from=build /src/dist /usr/share/nginx/html
USER 1001

EXPOSE 9443
ENTRYPOINT ["/usr/local/bin/ruptura-console-plugin"]
