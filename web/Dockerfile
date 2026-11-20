FROM node:22-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --engine-strict=false
COPY . .
RUN npm run build

# Caddy: sajikan SPA statis + reverse proxy /api ke backend + HTTPS otomatis
FROM caddy:2-alpine
COPY --from=build /app/build /srv
COPY Caddyfile /etc/caddy/Caddyfile
