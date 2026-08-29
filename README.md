> **⚠️ DISCLAIMER:** Discord live screensharing was restricted/blocked in Brazil, so I built my own self-hosted, ultra-low latency, native Wayland screen & audio streaming stack.

# 🖥️ Wayland Ultra-Low Latency Live Streamer

A high-performance, containerized screen and audio streaming solution engineered specifically for Linux **Wayland** desktops (Hyprland, Sway, GNOME, KDE Plasma). Delivers **sub-200ms ultra-low latency** live video with intelligent per-application audio isolation and zero port-forwarding via Cloudflare Tunnels.

---

## ⚡ Highlights

- **Ultra-Low Latency (<200ms)**: Stream live screen and game audio with virtually zero perceptible delay using MPEG-TS over WebSocket and WebRTC.
- **Native Wayland Screen Capture**: Directly interfaces with `xdg-desktop-portal` and PipeWire via D-Bus for zero-overhead hardware capture.
- **Smart Audio Router & Voice Isolation**:
  - Automatically isolates desktop and game audio.
  - Blacklists voice chat applications (**Discord, Vesktop, Slack, Zoom, Teams**) so voice call participants do not hear themselves echoing.
  - Generates continuous silence-clock timestamps to prevent muxer buffer deadlocks when no audio is playing.
- **Modern Web Player**:
  - Custom HTML5 player powered by `mpegts.js`.
  - Built-in interactive **volume slider** (0–100%) with volume memory (`localStorage`).
  - One-click mute/unmute and dynamic speaker indicators (`🔊`, `🔉`, `🔈`, `🔇`).
  - Aggressive buffer synchronization to maintain the live edge without stuttering.
- **Global Public Access (Cloudflare Tunnel)**:
  - Accessible securely over public HTTPS (`wss://`) without exposing open router ports or configuring complex NAT traversal.
- **Three Modular Streaming Backends**:
  1. `ws/` **(Default & Recommended)**: WebSocket MPEG-TS streaming with full Cloudflare Tunnel proxying and universal browser support.
  2. `webrtc/`: Direct P2P WebRTC broadcast powered by Pion Go (<150ms delay).
  3. `hls/`: Low-Latency HLS (LL-HLS) fallback for maximum compatibility.

---

## 🏗️ Architecture

```
[ Wayland Compositor (Hyprland / GNOME / KDE) ]
                       │
             xdg-desktop-portal (D-Bus)
                       │
                 PipeWire Stream
                       │
                       ▼
┌─────────────────────────────────────────────────────────┐
│                   Docker Container                      │
│                                                         │
│  [PipeWire / PulseAudio] ──► [Go Audio Router & Filter] │
│                                  │ (Isolate Discord)    │
│  [GStreamer x264enc] ◄───────────┘                      │
│           │                                             │
│      MPEG-TS Muxer (188-byte aligned)                   │
│           │                                             │
│      Go WebSocket Hub (Non-blocking per-client pump)    │
│           │                                             │
└───────────┼─────────────────────────────────────────────┘
            │
            ├──────────────► Local Player (http://localhost:8080)
            │
    [Cloudflare Tunnel (cloudflared)]
            │
            ▼
    Public Web Viewers (https://stream.yourdomain.com)
```

---

## 🚀 Quick Start

### 1. Prerequisites

- **Linux** with **Wayland** (Hyprland, Sway, GNOME Wayland, KDE Plasma Wayland).
- **PipeWire** & **WirePlumber** (or `pipewire-media-session`).
- **Docker** & **Docker Compose**.
- (Optional) **Cloudflare Tunnel Token** for public sharing.

### 2. Configuration

Copy the example environment file:

```bash
cp .env.example .env
```

Edit `.env` to configure your settings:

```bash
# Cloudflare Tunnel Token (optional, for public URL)
CLOUDFLARE_TUNNEL_TOKEN=your_token_here

# Apps to filter out from stream (prevents echo in Discord calls)
AUDIO_BLACKLIST=discord,Discord,vesktop,webcord,slack,zoom,teams

# Include microphone (false = only desktop/game audio)
INCLUDE_MIC=false

# Video encoder mode:
# - gpu   : Hardware-accelerated encoding (VA-API for AMD/Intel, NVENC for NVIDIA) [Default & Recommended]
# - cpu   : CPU software encoding via x264 (fallback or CPU-only)
ENCODER=gpu

# Video output settings
FRAMERATE=60
TARGET_WIDTH=1920
TARGET_HEIGHT=1080
VIDEO_BITRATE=6000k
```

### 3. Launch Streamer (Choose GPU or CPU)

You can launch using hardware acceleration (**GPU**) or software (**CPU**) directly via CLI flags or your `.env` configuration:

```bash
# Launch default WebSocket streamer with GPU hardware acceleration
./run.sh --gpu

# Launch with CPU-only encoding (software x264)
./run.sh --cpu

# Run specific backend with GPU or CPU
./run.sh ws --gpu
./run.sh webrtc --gpu
./run.sh hls --cpu
```

When prompted on your desktop, select the screen or window you want to share.

### 4. Watch the Stream

- **Local Network**: `http://localhost:8080`
- **Public Domain**: `https://stream.yourdomain.com` (configured in your Cloudflare Zero Trust dashboard)

---

## 🎛️ Running Specific Streaming Engines

You can launch any of the three independent streaming implementations with `--gpu` or `--cpu`:

| Engine | Launcher Script | Protocol | Latency | Cloudflare Tunnel Support |
| :--- | :--- | :--- | :--- | :--- |
| **WebSocket (Recommended)** | `./run_ws.sh [--gpu\|--cpu]` | MPEG-TS over WS | **~200ms** | ✅ Full Support |
| **WebRTC** | `./run_webrtc.sh [--gpu\|--cpu]` | WebRTC (Pion) | **~150ms** | Direct / Local / P2P |
| **Low-Latency HLS** | `./run_hls.sh [--gpu\|--cpu]` | HLS (.m3u8) | **~2-3s** | ✅ Full Support |

---

## 🚀 GPU Hardware Acceleration vs CPU

- **GPU Mode (`ENCODER=gpu` / `--gpu`)**:
  - Uses Linux **VA-API** (`vaapih264enc` / `h264_vaapi`) for AMD Radeon and Intel GPUs, or **NVENC** (`nvh264enc` / `h264_nvenc`) for NVIDIA GPUs.
  - Zero CPU overhead, preserving all CPU threads for games and high-framerate tasks.
- **CPU Mode (`ENCODER=cpu` / `--cpu`)**:
  - Uses `x264enc` (GStreamer) and `libx264` (FFmpeg) configured with `ultrafast` zerolatency profiles.
  - Universally compatible on any machine without requiring GPU pass-through or render permissions.

---

## 🔈 Audio Modes: Passive Mirror vs Voice Isolation

You can choose between two audio capture strategies:

### 1. Passive Headphone Mirror Mode (`--mirror` / `AUDIO_ROUTING=false`) [Recommended]
* **How it works**: Uses PipeWire's built-in sink monitor (`<default-sink>.monitor` or `easyeffects_sink.monitor`).
* **Zero System Touch**: Does **not** change your default audio device, does **not** move applications (Chrome, games, Spotify remain exactly where you put them), and does **not** interfere with EasyEffects.
* **Result**: Everything you hear in your headphones is mirrored into the live stream with 0ms added latency.

```bash
# Launch with passive mirror mode
./run.sh --gpu --mirror
```

### 2. Voice Isolation Mode (`--isolate` / `AUDIO_ROUTING=true`)
* **How it works**: Creates a virtual `stream_sink` and dynamically scans active audio streams every second:
  - **Games / YouTube / Chrome / Music** $\rightarrow$ Routed into `stream_sink` (streamed to viewers) and looped back to headphones.
  - **Discord / Slack / Zoom Voice Calls** $\rightarrow$ Kept strictly on physical headphones (excluded from stream).

```bash
# Launch with voice isolation mode
./run.sh --gpu --isolate
```

---

## 🛠️ Project Structure

```
.
├── README.md               # Project documentation & disclaimer
├── run.sh                  # Default streamer launcher (runs WebSocket streamer)
├── run_ws.sh               # WebSocket streamer launcher
├── run_webrtc.sh           # WebRTC streamer launcher
├── run_hls.sh              # HLS streamer launcher
├── .env.example            # Example configuration template
├── .gitignore              # Git ignore rules for secrets and temp files
│
├── ws/                     # [MPEG-TS over WebSocket Streamer]
│   ├── cmd/streamer/       # Application entry point
│   ├── pkg/
│   │   ├── audio/          # PulseAudio / PipeWire router & blacklist filter
│   │   ├── config/         # Environment variable parser
│   │   ├── pipeline/       # GStreamer pipeline with silence-clock mixer
│   │   ├── portal/         # Wayland XDG Desktop Portal D-Bus client
│   │   └── server/         # Go HTTP & WebSocket Hub (non-blocking pump)
│   ├── web/                # Web player UI (HTML5 + mpegts.js + volume slider)
│   └── docker-compose.yml  # Container orchestration & Cloudflare Tunnel
│
├── webrtc/                 # [WebRTC Streamer]
│   ├── pkg/webrtc/         # Pion WebRTC broadcaster and track injector
│   └── web/                # WebRTC player with SDP HTTP signaling
│
└── hls/                    # [LL-HLS Streamer]
    ├── nginx.conf          # Low-latency HLS caching & CORS configuration
    └── web/                # Hls.js web player
```

---

## 📄 License

MIT License. Feel free to modify, host, and distribute.
