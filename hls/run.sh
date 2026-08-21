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
    TUNNEL_MODE="Custom Token Tunnel (${DOMAIN:-Configured Cloudflare Domain})"
else
    export CLOUDFLARE_TUNNEL_COMMAND="tunnel --no-autoupdate --url http://streamer:8080"
    TUNNEL_MODE="Automatic Quick Tunnel (TryCloudflare - Free / No Account Needed)"
fi

echo "======================================================="
echo "   Go Wayland HLS Live Stream + Cloudflare Tunnel"
echo "======================================================="
echo "--> Tunnel Mode: ${TUNNEL_MODE}"
echo "    [Local Player URL : http://localhost:8080]"
echo "    [Cloudflare URL   : Auto-generated trycloudflare.com URL will be logged below]"
echo ""

cleanup() {
    echo -e "\n[!] Stopping Docker HLS streaming containers..."
    cd "$SCRIPT_DIR" && docker compose down
    exit 0
}

trap cleanup INT TERM

cd "$SCRIPT_DIR"
docker compose up --build

