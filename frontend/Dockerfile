# muara.bi web app (Next.js standalone server).
#   docker build -f frontend/Dockerfile -t muara-bi-web:latest ./frontend
# With NODE_IMAGE=public.ecr.aws/docker/library/node:24-alpine as a --build-arg
# if Docker Hub is unreachable from the build host.

ARG NODE_IMAGE=node:24-alpine

# ------------------------------------------------------------------ deps -----
FROM ${NODE_IMAGE} AS deps
WORKDIR /app
COPY package.json package-lock.json ./
# `ci` honours the lockfile exactly, so the image matches local development.
RUN npm ci --no-audit --no-fund

# ----------------------------------------------------------------- build -----
FROM ${NODE_IMAGE} AS build
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
# The /api/* rewrite lives in .next/routes-manifest.json, which Next writes at
# build time: a runtime API_BASE_URL cannot change it. So the target has to be a
# build argument. Docker Compose passes http://api:8080, the compose-network name
# of the Go service; the default keeps local `npm run dev` behaviour.
ARG API_BASE_URL=http://127.0.0.1:8080
ENV API_BASE_URL=${API_BASE_URL}
RUN npm run build

# --------------------------------------------------------------- runtime -----
FROM ${NODE_IMAGE} AS runtime
WORKDIR /app

RUN addgroup -g 10001 nodejs \
 && adduser -S -u 10001 -G nodejs nextjs

ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    HOSTNAME=0.0.0.0 \
    PORT=3000

# Standalone carries the trimmed server plus only the modules it imports.
COPY --from=build --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=build --chown=nextjs:nodejs /app/.next/static ./.next/static
COPY --from=build --chown=nextjs:nodejs /app/public ./public

USER nextjs
EXPOSE 3000

# busybox wget ships with alpine, so this adds no packages.
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD wget -q --spider http://127.0.0.1:3000/ || exit 1

CMD ["node", "server.js"]
