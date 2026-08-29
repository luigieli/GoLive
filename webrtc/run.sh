#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ -f "$SCRIPT_DIR/../.env" ]; then
    set -a
    source "$SCRIPT_DIR/../.env"
    set +a
elif [ -f "$SCRIPT_DIR/.env" ]; then
    set -a
    source "$SCRIPT_DIR/.env"
    set +a
fi

TOKEN="${CLOUDFLARE_TUNNEL_TOKEN:-}"
if [ -n "$TOKEN" ] && [ "$TOKEN" != "your_cloudflare_tunnel_token_here" ] && [ "$TOKEN" != "your_token_here" ]; then
    export CLOUDFLARE_TUNNEL_COMMAND="tunnel --no-autoupdate run --token ${TOKEN}"
    TUNNEL_MODE="Custom Cloudflare Token Tunnel"
else
    export CLOUDFLARE_TUNNEL_COMMAND="tunnel --no-autoupdate --url http://streamer:8080"
    TUNNEL_MODE="Automatic Quick Tunnel (TryCloudflare - Free / No Account Needed)"
fi

for arg in "$@"; do
    case "$arg" in
        --cpu|-cpu|-c)
            export ENCODER="cpu"
            ;;
        --gpu|-gpu|-g)
            export ENCODER="gpu"
            ;;
        --vaapi|vaapi)
            export ENCODER="vaapi"
            ;;
        --nvenc|nvenc)
            export ENCODER="nvenc"
            ;;
        --mirror|-m|mirror)
            export AUDIO_ROUTING="false"
            export AUDIO_SOURCE="mirror"
            ;;
        --isolate|-i|isolate)
            export AUDIO_ROUTING="true"
            export AUDIO_SOURCE="stream_sink.monitor"
            ;;
    esac
done

ENCODER="${ENCODER:-gpu}"
AUDIO_ROUTING="${AUDIO_ROUTING:-true}"

echo "======================================================="
echo "   ⚡ Go Wayland WebRTC Streamer + Cloudflare Tunnel"
echo "======================================================="
if [ "$ENCODER" = "cpu" ] || [ "$ENCODER" = "x264" ]; then
    echo "--> Video Encoder : CPU Software (x264enc)"
else
    echo "--> Video Encoder : GPU Accelerated ($ENCODER / VA-API / NVENC)"
fi
if [ "$AUDIO_ROUTING" = "false" ]; then
    echo "--> Audio Mode    : Passive Headphone Mirror (Headphones & Apps Untouched)"
else
    echo "--> Audio Mode    : Voice Isolation (Discord/Slack filtered from stream)"
fi
echo "--> Tunnel Mode   : ${TUNNEL_MODE}"
echo "    [Local Player URL : http://localhost:8080]"
echo ""

cleanup() {
    echo -e "\n[!] Stopping Docker WebRTC streaming containers..."
    cd "$SCRIPT_DIR" && docker compose down 2>/dev/null || true
    exit 0
}

trap cleanup INT TERM

cd "$SCRIPT_DIR"
docker compose up --build

