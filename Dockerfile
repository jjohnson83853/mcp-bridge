# Multi-stage build: static Go binary, minimal non-root runtime.
# Mirrors the stack's other Go MCP servers (see ../mcp-codebase-searcher).
#
# The operator document is NOT baked into the image: it is mounted read-only at
# runtime (see compose.yaml), so the same image serves any document.

FROM golang:1.25.0-alpine3.22 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/mcp-bridge ./cmd/mcp-bridge

FROM alpine:3.22

# ca-certificates: the embedding provider (embedding.base_url) is reached over
# HTTPS. The upstream MCP servers are plain HTTP on the stack network. Busybox
# wget backs the healthcheck below.
RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=build /out/mcp-bridge /app/mcp-bridge

# storage.directory ("./data" in the document) resolves against the process
# working directory, so private state lives at /app/data. Create it owned by the
# runtime UID: an empty named volume mounted here inherits that owner. Every
# write the bridge performs is confined to this directory.
RUN mkdir -p /app/data && chown 1000:1000 /app/data

USER 1000:1000
EXPOSE 8080 9090

# /livez is the operations listener's process check. /readyz tracks per-upstream
# readiness and would flap this container whenever an upstream is down.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD ["wget", "-q", "-O", "/dev/null", "http://127.0.0.1:9090/livez"]

ENTRYPOINT ["/app/mcp-bridge"]
CMD ["--config", "/app/configs/config.yaml"]
