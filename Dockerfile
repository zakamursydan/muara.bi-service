# muara.bi core API + Asynq worker.
#
# No "# syntax=" directive: BuildKit's embedded parser handles everything here,
# and pulling docker/dockerfile for each build would make the build depend on
# Docker Hub even when the base images are already cached.
#
# Build from the repository root so the context stays small:
#   docker build -f Dockerfile -t muara-bi-core:latest .
# If Docker Hub is unreachable from the build host, point the base image at a
# mirror (compose reads the same value from GO_IMAGE in .env):
#   docker build --build-arg GO_IMAGE=public.ecr.aws/docker/library/golang:1.27-bookworm -f Dockerfile .
#
# The binary links DuckDB's C++ static archive, so the image is built for the
# host architecture only: go-duckdb ships libduckdb.a for linux_amd64 and
# linux_arm64, and cross-compiling CGO is not configured here.

ARG GO_IMAGE=golang:1.27-bookworm

# ---------------------------------------------------------------- build ------
FROM ${GO_IMAGE} AS build

# libduckdb.a needs a real C++ linker, and DuckDB extensions are dlopen'd, so
# the executable must keep its dynamic symbol table: -extldflags=-Wl,-E.
ENV CGO_ENABLED=1
WORKDIR /src

# Resolve modules first so source-only edits reuse the download cache.
COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./

RUN set -eux; \
    ldflags='-s -w -extldflags=-Wl,-E'; \
    go build -trimpath -ldflags="$ldflags" -o /out/api    ./cmd/api; \
    go build -trimpath -ldflags="$ldflags" -o /out/worker ./cmd/worker; \
    go build -trimpath -o /out/duckdb-bundle ./cmd/duckdb-bundle

# ------------------------------------------------------------- extensions ----
# DuckDB downloads the connector binaries itself, which is what chooses the
# correct <version>/<platform> subdirectory. Without this stage a container that
# has no internet access cannot ATTACH Postgres, MySQL or SQLite sources, and
# every federated query fails with a confusing Catalog Error.
FROM build AS extensions
RUN /out/duckdb-bundle -out /opt/duckdb/extensions

# --------------------------------------------------------------- runtime -----
# glibc, not musl: the bundled libduckdb.a is built against libstdc++/glibc.
FROM debian:bookworm-slim AS runtime

RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl libstdc++6 tzdata \
 && rm -rf /var/lib/apt/lists/*

RUN groupadd --gid 10001 muara \
 && useradd --uid 10001 --gid muara --create-home --shell /usr/sbin/nologin muara

WORKDIR /app

COPY --from=build /out/api /out/worker /usr/local/bin/
# Read-only for the service user: the app treats INSTALL as best effort and only
# requires LOAD to succeed, which finds these files via DUCKDB_EXTENSION_DIRECTORY.
COPY --from=extensions /opt/duckdb/extensions /opt/duckdb/extensions

ENV DUCKDB_EXTENSION_DIRECTORY=/opt/duckdb/extensions \
    APP_HTTP_PORT=8080 \
    TZ=UTC \
    HOME=/home/muara

EXPOSE 8080

USER 10001:10001

HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
    CMD curl -fsS "http://127.0.0.1:${APP_HTTP_PORT}/health" || exit 1

# docker compose overrides this with /usr/local/bin/worker for the queue pool.
CMD ["/usr/local/bin/api"]
