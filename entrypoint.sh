#!/bin/bash
set -e

echo "=================================================="
echo " Starting HLS Screen & Audio Streaming Container "
echo "=================================================="

mkdir -p /tmp/hls /var/log/nginx
rm -rf /tmp/hls/*

# Start NGINX
echo "Starting Nginx web server on port 8080..."
nginx

cleanup() {
    echo "Stopping streamer..."
    kill -TERM "$CAPTURE_PID" 2>/dev/null || true
    nginx -s stop 2>/dev/null || true
    exit 0
}
trap cleanup SIGTERM SIGINT

# Run python Wayland Portal ScreenCast & PipeWire HLS Pipeline
python3 /usr/local/bin/capture.py &
CAPTURE_PID=$!

wait "$CAPTURE_PID"
