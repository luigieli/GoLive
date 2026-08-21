#!/bin/bash
set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

MODE="${1:-ws}"

case "$MODE" in
    ws|websocket)
        echo "[*] Starting Ultra-Low Latency WebSocket Streamer (<200ms, Cloudflare Tunnel)..."
        exec "$SCRIPT_DIR/ws/run.sh"
        ;;
    webrtc)
        echo "[*] Starting Ultra-Low Latency WebRTC Streamer (P2P / Direct UDP)..."
        exec "$SCRIPT_DIR/webrtc/run.sh"
        ;;
    hls)
        echo "[*] Starting HLS Live Streamer (HTTP Chunked)..."
        exec "$SCRIPT_DIR/hls/run.sh"
        ;;
    *)
        echo "Usage: ./run.sh [ws|webrtc|hls]"
        echo "  - ws     : WebSocket ultra-low latency (<200ms) over Cloudflare Tunnel [Default]"
        echo "  - webrtc : WebRTC direct peer-to-peer / UDP streaming (<100ms)"
        echo "  - hls    : Standard HTTP Live Streaming"
        exit 1
        ;;
esac
