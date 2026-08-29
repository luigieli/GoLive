FROM golang:1.24-bookworm AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/streamer ./cmd/streamer

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    gstreamer1.0-tools \
    gstreamer1.0-pipewire \
    gstreamer1.0-plugins-base \
    gstreamer1.0-plugins-good \
    gstreamer1.0-plugins-bad \
    gstreamer1.0-plugins-ugly \
    gstreamer1.0-vaapi \
    mesa-va-drivers \
    libva2 \
    libva-drm2 \
    intel-media-va-driver \
    vainfo \
    libpulse0 \
    pulseaudio-utils \
    dbus \
    libdbus-1-3 \
    && rm -rf /var/lib/apt/lists/*

RUN groupadd -g 1000 streamer && \
    useradd -u 1000 -g streamer -m streamer

COPY --from=builder /app/streamer /usr/local/bin/streamer
RUN chmod +x /usr/local/bin/streamer

COPY web /home/streamer/web

USER streamer
WORKDIR /home/streamer

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/streamer"]
