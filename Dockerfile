# syntax=docker/dockerfile:1
FROM golang:1.26-alpine AS builder
WORKDIR /src

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/jozmozdns ./cmd/jozmozdns

FROM alpine:3.20
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /out/jozmozdns /app/jozmozdns

RUN mkdir -p /app/data /app/certs

EXPOSE 53/udp 53/tcp 80/tcp 443/tcp 853/tcp 8080/tcp 9443/tcp
VOLUME ["/app/data", "/app/certs"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q -O /dev/null "http://127.0.0.1:${JOZMOZDNS_HEALTH_PORT:-8080}/api/v1/version" || exit 1

ENTRYPOINT ["/app/jozmozdns", "-daemon", \
    "-config", "/app/data/config.json", \
    "-db", "/app/data/data.db", \
    "-key", "/app/data/master.key"]
