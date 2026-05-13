# ── Stage 1: build Svelte app ────────────────────────────────────────────────
FROM node:20-alpine AS build

WORKDIR /app

COPY package.json package-lock.json* ./
RUN npm ci

COPY . .
RUN npm run build

# ── Stage 2: serve with nginx ─────────────────────────────────────────────────
FROM nginx:1.25-alpine

# Remove default nginx config
RUN rm -f /etc/nginx/conf.d/default.conf

# Copy built assets
COPY --from=build /app/dist /usr/share/nginx/html

# Copy nginx config template and entrypoint
COPY nginx.conf.template /tmp/nginx.conf.template
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

# Default backend — override with RUPTURA_BACKEND_URL env var in Helm/k8s
ENV RUPTURA_BACKEND_URL=http://ruptura:8080

EXPOSE 80

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://localhost/health || exit 1

ENTRYPOINT ["/docker-entrypoint.sh"]
