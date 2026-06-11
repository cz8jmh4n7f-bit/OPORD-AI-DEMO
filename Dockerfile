# syntax=docker/dockerfile:1
# Multi-target build for the OPORD AI-first stack. One Go build stage produces the
# api, the worker, and a goose migrate binary; all are static (CGO disabled), so
# the runtime images are tiny alpine. The AI-first build governs AI access and
# does NOT provision infrastructure, so no tofu / ansible is bundled.
#
# Built per-service via deployments/ai-compose.yml (target: api|worker|migrate).

FROM golang:1.25-bookworm AS gobuild
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=0 GOOS=linux
RUN go build -trimpath -ldflags="-s -w" -o /out/opord-api ./cmd/api \
 && go build -trimpath -ldflags="-s -w" -o /out/opord-worker ./cmd/worker \
 && GOBIN=/out go install github.com/pressly/goose/v3/cmd/goose@latest

# --- opord-api: HTTP API + enqueue + reapers ---
FROM alpine:3.20 AS api
RUN apk add --no-cache ca-certificates wget && adduser -D -u 10001 opord
COPY --from=gobuild /out/opord-api /usr/local/bin/opord-api
USER opord
EXPOSE 8080
ENTRYPOINT ["opord-api"]

# --- opord-worker: River durable job pool ---
FROM alpine:3.20 AS worker
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 opord
COPY --from=gobuild /out/opord-worker /usr/local/bin/opord-worker
USER opord
ENTRYPOINT ["opord-worker"]

# --- migrate: goose runner (compose supplies the args) ---
FROM alpine:3.20 AS migrate
RUN apk add --no-cache ca-certificates
COPY --from=gobuild /out/goose /usr/local/bin/goose
COPY migrations /migrations
ENTRYPOINT ["goose"]
