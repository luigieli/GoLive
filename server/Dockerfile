FROM golang:1.24-bullseye AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY utils/ utils/
COPY server/ server/

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/server ./server/cmd/main.go

FROM debian:bullseye-slim

WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/server /usr/local/bin/server
COPY server/web /app/web

ENV PORT=8080 \
    WEB_DIR=/app/web

EXPOSE 8080 50000-50050/udp

ENTRYPOINT ["/usr/local/bin/server"]
