#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Load .env if present
if [ -f "$SCRIPT_DIR/.env" ]; then
    set -a
    source "$SCRIPT_DIR/.env"
    set +a
fi

ACTION="all"
ENCODER_FLAG=""

for arg in "$@"; do
    case "$arg" in
        all)
            ACTION="all"
            ;;
        server)
            ACTION="server"
            ;;
        client)
            ACTION="client"
            ;;
        legacy-ws|ws)
            ACTION="legacy-ws"
            ;;
        legacy-webrtc|webrtc)
            ACTION="legacy-webrtc"
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
        --mirror|-m|mirror)
            export AUDIO_ROUTING="false"
            export AUDIO_SOURCE="mirror"
            ;;
        --isolate|-i|isolate)
            export AUDIO_ROUTING="true"
            export AUDIO_SOURCE="stream_sink.monitor"
            ;;
        --help|-h)
            echo "⚡ Go Wayland & OBS Streaming Hub"
            echo ""
            echo "Usage: ./run.sh [COMMAND] [OPTIONS]"
            echo ""
            echo "Commands:"
            echo "  all              Launch Server (Docker + Tunnel) and local Capture Client [Default]"
            echo "  server           Launch only the Distribution Server + Cloudflare Tunnel in Docker"
            echo "  client           Launch only the local Wayland Capture Client"
            echo "  legacy-ws        Run monolithic WebSocket streamer container"
            echo "  legacy-webrtc    Run monolithic WebRTC streamer container"
            echo ""
            echo "Options:"
            echo "  --gpu, -g        Hardware accelerated encoding (VA-API / NVENC) [Default]"
            echo "  --cpu, -c        CPU software encoding (libx264)"
            echo "  --mirror, -m     Passive headphone mirror (direct, untouched)"
            echo "  --isolate, -i    Voice isolation (filters Discord/Slack from stream)"
            echo ""
            echo "Examples:"
            echo "  ./run.sh                  # Launch full application (Server + Client) with GPU & mirror"
            echo "  ./run.sh server           # Run server hub for OBS Studio streaming"
            echo "  ./run.sh client --mirror  # Run local capture client connecting to server"
            exit 0
            ;;
    esac
done

if [ -n "$ENCODER_FLAG" ]; then
    export ENCODER="$ENCODER_FLAG"
fi

export ENCODER="${ENCODER:-gpu}"
export AUDIO_ROUTING="${AUDIO_ROUTING:-false}"
export AUDIO_SOURCE="${AUDIO_SOURCE:-mirror}"

TOKEN="${CLOUDFLARE_TUNNEL_TOKEN:-}"
if [ -n "$TOKEN" ] && [ "$TOKEN" != "your_cloudflare_tunnel_token_here" ] && [ "$TOKEN" != "your_token_here" ]; then
    export CLOUDFLARE_TUNNEL_COMMAND="tunnel --no-autoupdate run --token ${TOKEN}"
    TUNNEL_MODE="Custom Cloudflare Token Tunnel"
else
    export CLOUDFLARE_TUNNEL_COMMAND="tunnel --no-autoupdate --url http://server:8080"
    TUNNEL_MODE="Automatic Quick Tunnel (TryCloudflare - Free / No Account Needed)"
fi

echo "======================================================="
echo "   ⚡ Go Wayland Streaming Architecture"
echo "======================================================="
echo "--> Action        : ${ACTION}"
echo "--> Video Encoder : ${ENCODER}"
if [ "$AUDIO_ROUTING" = "false" ]; then
    echo "--> Audio Mode    : Passive Headphone Mirror"
else
    echo "--> Audio Mode    : Voice Isolation (Discord filtered)"
fi
echo "--> Tunnel Mode   : ${TUNNEL_MODE}"
echo "======================================================="
echo ""

cleanup() {
    echo -e "\n[*] Stopping streaming services..."
    cd "$SCRIPT_DIR"
    docker compose down 2>/dev/null || true
    exit 0
}
trap cleanup INT TERM

case "$ACTION" in
    all)
        echo "[1/2] Starting Stream Server & Cloudflare Tunnel in Docker..."
        docker compose up -d --build server tunnel
        echo "--> Server is live at http://localhost:8080"
        echo "--> OBS WHIP Ingest : http://localhost:8080/whip"
        echo "--> Cloudflare logs :"
        docker compose logs -f tunnel &
        LOG_PID=$!

        sleep 2
        echo ""
        echo "[2/2] Starting local Wayland Capture Client..."
        export SERVER_URL="http://localhost:8080/api/publish"
        go run "$SCRIPT_DIR/client/cmd/main.go" || true

        kill $LOG_PID 2>/dev/null || true
        cleanup
        ;;

    server)
        echo "[*] Starting Stream Server & Cloudflare Tunnel..."
        echo "--> Server URL       : http://localhost:8080"
        echo "--> OBS WHIP Ingest  : http://localhost:8080/whip"
        echo "--> Client Ingest    : http://localhost:8080/api/publish"
        echo ""
        docker compose up --build server tunnel
        ;;

    client)
        echo "[*] Starting local Wayland Capture Client..."
        export SERVER_URL="${SERVER_URL:-http://localhost:8080/api/publish}"
        go run "$SCRIPT_DIR/client/cmd/main.go"
        ;;

    legacy-ws)
        echo "[*] Launching legacy monolithic WebSocket streamer..."
        exec "$SCRIPT_DIR/ws/run.sh"
        ;;

    legacy-webrtc)
        echo "[*] Launching legacy monolithic WebRTC streamer..."
        exec "$SCRIPT_DIR/webrtc/run.sh"
        ;;
esac
