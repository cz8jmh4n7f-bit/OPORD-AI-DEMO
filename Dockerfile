# syntax=docker/dockerfile:1
# Multi-target build for the OPORD AI governance stack. One Go build stage produces
# the api and a goose migrate binary; both are static (CGO disabled), so the
# runtime images are tiny alpine. OPORD governs AI access and does NOT provision
# infrastructure, so no tofu / ansible is bundled.
#
# Built per-service via deployments/ai-compose.yml (target: api|migrate).

FROM golang:1.25-bookworm AS gobuild
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=0 GOOS=linux
RUN go build -trimpath -ldflags="-s -w" -o /out/opord-api ./cmd/api \
 && GOBIN=/out go install github.com/pressly/goose/v3/cmd/goose@latest

# --- opord-api: HTTP API + enqueue + reapers ---
FROM alpine:3.20 AS api
RUN apk add --no-cache ca-certificates wget && adduser -D -u 10001 opord
COPY --from=gobuild /out/opord-api /usr/local/bin/opord-api
USER opord
EXPOSE 8080
ENTRYPOINT ["opord-api"]

# --- migrate: goose runner (compose supplies the args) ---
FROM alpine:3.20 AS migrate
RUN apk add --no-cache ca-certificates
COPY --from=gobuild /out/goose /usr/local/bin/goose
COPY migrations /migrations
ENTRYPOINT ["goose"]
