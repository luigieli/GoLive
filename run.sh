#!/bin/bash
set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

MODE="${1:-webrtc}"

case "$MODE" in
    hls)
        echo "[*] Starting HLS Live Streamer..."
        exec "$SCRIPT_DIR/hls/run.sh"
        ;;
    webrtc)
        echo "[*] Starting Ultra-Low Latency WebRTC Streamer..."
        exec "$SCRIPT_DIR/webrtc/run.sh"
        ;;
    *)
        echo "Usage: ./run.sh [webrtc|hls]"
        echo "  - webrtc : Ultra-low latency real-time streaming (<150ms delay) [Default]"
        echo "  - hls    : Standard HTTP Live Streaming"
        exit 1
        ;;
esac
