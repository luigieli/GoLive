#!/bin/bash
set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

MODE="ws"
ENCODER_FLAG=""

for arg in "$@"; do
    case "$arg" in
        ws|websocket)
            MODE="ws"
            ;;
        webrtc)
            MODE="webrtc"
            ;;
        hls)
            MODE="hls"
            ;;
        --cpu|-cpu|-c)
            ENCODER_FLAG="cpu"
            ;;
        --gpu|-gpu|-g)
            ENCODER_FLAG="gpu"
            ;;
        --vaapi|vaapi)
            ENCODER_FLAG="vaapi"
            ;;
        --nvenc|nvenc)
            ENCODER_FLAG="nvenc"
            ;;
        --help|-h)
            echo "Wayland Screen & Audio Live Streamer"
            echo ""
            echo "Usage: ./run.sh [MODE] [OPTIONS]"
            echo ""
            echo "Modes:"
            echo "  ws, websocket    Ultra-low latency WebSocket streaming (<200ms) [Default]"
            echo "  webrtc           WebRTC direct P2P / UDP streaming (<150ms)"
            echo "  hls              Low-latency HTTP Live Streaming"
            echo ""
            echo "Encoder Options:"
            echo "  --gpu, -g        Hardware accelerated encoding (VA-API / NVENC) [Default]"
            echo "  --cpu, -c        CPU software encoding (libx264 / x264enc)"
            echo "  --vaapi          Force VA-API GPU encoding (AMD Radeon / Intel)"
            echo "  --nvenc          Force NVIDIA NVENC GPU encoding"
            echo ""
            echo "Examples:"
            echo "  ./run.sh --gpu           # Launch WebSocket streamer with GPU"
            echo "  ./run.sh --cpu           # Launch WebSocket streamer with CPU only"
            echo "  ./run.sh webrtc --gpu    # Launch WebRTC streamer with GPU"
            echo "  ./run.sh hls --cpu       # Launch HLS streamer with CPU only"
            exit 0
            ;;
        *)
            echo "Unknown argument: $arg"
            echo "Run './run.sh --help' for usage instructions."
            exit 1
            ;;
    esac
done

if [ -n "$ENCODER_FLAG" ]; then
    export ENCODER="$ENCODER_FLAG"
fi

case "$MODE" in
    ws)
        exec "$SCRIPT_DIR/ws/run.sh"
        ;;
    webrtc)
        exec "$SCRIPT_DIR/webrtc/run.sh"
        ;;
    hls)
        exec "$SCRIPT_DIR/hls/run.sh"
        ;;
esac
