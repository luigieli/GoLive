#!/usr/bin/env bash
set -euo pipefail

# Always load .env from project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ -f "$SCRIPT_DIR/../.env" ]; then
    set -a
    source "$SCRIPT_DIR/../.env"
    set +a
fi

echo "======================================================="
echo "   ⚡ Go Wayland WebSocket Streamer + Cloudflare Tunnel"
echo "======================================================="
echo "--> Building & Starting WebSocket Streaming Stack inside Docker..."
echo "    [Cloudflare Stream URL : https://stream.luigieli.com]"
echo "    [Local Player URL      : http://localhost:8080]"
echo ""

cleanup() {
    echo -e "\n[!] Stopping Docker WebSocket streaming containers..."
    docker compose down
    exit 0
}

trap cleanup INT TERM

cd "$SCRIPT_DIR"
docker compose up --build
