# Build Stage
FROM golang:1.24-bookworm AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY utils/ utils/
COPY client/ client/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/client ./client/cmd/main.go

# Runner Stage
FROM debian:bookworm-slim
ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    gstreamer1.0-tools \
    gstreamer1.0-pipewire \
    gstreamer1.0-plugins-base \
    gstreamer1.0-plugins-good \
    gstreamer1.0-plugins-bad \
    gstreamer1.0-plugins-ugly \
    gstreamer1.0-libav \
    gstreamer1.0-vaapi \
    mesa-va-drivers \
    libva2 \
    libva-drm2 \
    intel-media-va-driver \
    vainfo \
    libpulse0 \
    pulseaudio-utils \
    pipewire-bin \
    dbus \
    libdbus-1-3 \
    && rm -rf /var/lib/apt/lists/*

RUN groupadd -g 1000 streamer && \
    useradd -u 1000 -g streamer -m streamer

COPY --from=builder /app/client /usr/local/bin/client
RUN chmod +x /usr/local/bin/client

USER streamer
WORKDIR /home/streamer

ENTRYPOINT ["/usr/local/bin/client"]
