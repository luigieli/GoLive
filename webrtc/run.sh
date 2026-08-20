#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "======================================================="
echo "   ⚡ Go Wayland WebRTC Streamer + Cloudflare Tunnel  "
echo "======================================================="

# Load .env if present in root or current dir
if [ -f ../.env ]; then
    set -a
    source ../.env
    set +a
elif [ -f .env ]; then
    set -a
    source .env
    set +a
fi

cleanup() {
    echo ""
    echo "[!] Stopping Docker WebRTC streaming containers..."
    docker compose down 2>/dev/null || true
    exit 0
}
trap cleanup SIGINT SIGTERM EXIT

echo "--> Building & Starting WebRTC Streaming Stack inside Docker..."
echo "    [Cloudflare Stream URL : https://stream.luigieli.com]"
echo "    [Local Player URL      : http://localhost:8080]"
echo ""

docker compose up --build
