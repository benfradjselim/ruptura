# dist/ is pre-built by CI (npm ci && npm run build) before this Dockerfile runs.
# Skipping Node.js in Docker keeps multi-arch builds fast — no QEMU npm needed.
FROM nginx:1.25-alpine

RUN rm -f /etc/nginx/conf.d/default.conf \
    && chown nginx:nginx /etc/nginx/conf.d \
    && touch /var/run/nginx.pid \
    && chown nginx:nginx /var/run/nginx.pid \
    && chown -R nginx:nginx /var/cache/nginx /var/log/nginx

COPY dist /usr/share/nginx/html
COPY nginx.conf.template /tmp/nginx.conf.template
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

ENV RUPTURA_BACKEND_URL=http://ruptura:8080

EXPOSE 80

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://localhost/health || exit 1

ENTRYPOINT ["/docker-entrypoint.sh"]
