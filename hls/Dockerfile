# Build Stage
FROM golang:1.24-bookworm AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/streamer ./cmd/streamer

# Runtime Stage
FROM debian:bookworm-slim

# Install FFmpeg, GStreamer PipeWire plugins, PulseAudio & DBus libraries
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    ffmpeg \
    gstreamer1.0-tools \
    gstreamer1.0-pipewire \
    gstreamer1.0-plugins-base \
    gstreamer1.0-plugins-good \
    libpulse0 \
    pulseaudio-utils \
    dbus \
    libdbus-1-3 \
    && rm -rf /var/lib/apt/lists/*

# Create user matching host UID (1000)
RUN groupadd -g 1000 streamer && \
    useradd -u 1000 -g streamer -m streamer && \
    mkdir -p /tmp/hls && \
    chown -R streamer:streamer /tmp/hls

COPY --from=builder /app/streamer /usr/local/bin/streamer
RUN chmod +x /usr/local/bin/streamer

USER streamer
WORKDIR /home/streamer

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/streamer"]
