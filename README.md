> **⚠️ DISCLAIMER:** Discord live screensharing was restricted/blocked in Brazil, so I built my own self-hosted, ultra-low latency, native Wayland & OBS screen & audio streaming stack.

# 🖥️ Ultra-Low Latency Live Streaming Stack (Wayland & OBS Studio)

A high-performance, modular screen and audio streaming solution engineered for Linux **Wayland** desktops (Hyprland, Sway, GNOME, KDE Plasma) and **OBS Studio** (Windows & Linux). Delivers **sub-200ms ultra-low latency** live video with intelligent per-application audio isolation and zero port-forwarding via Cloudflare Tunnels.

---

## ⚡ Highlights

- **Decoupled Client/Server Architecture**:
  - **Server (Distribution Hub)**: Lightweight pure Go server (~25MB container) handling stream ingestion, WebSocket fan-out, Pion WebRTC broadcasting, and Cloudflare Tunnels.
  - **Clients (Producers)**: Supports multiple simultaneous ingest sources:
    - **Native Linux Wayland Go Client**: Zero-copy DMA-BUF capture, GPU VA-API/NVENC encoding, and PulseAudio voice isolation.
    - **OBS Studio (Windows & Linux)**: Native **WHIP (WebRTC)** or HTTP streaming with one click.
- **Ultra-Low Latency (<200ms)**: Stream live screen and game audio with virtually zero perceptible delay using MPEG-TS over WebSocket and WebRTC.
- **Native Wayland Screen Capture**: Directly interfaces with `xdg-desktop-portal` and PipeWire via D-Bus for zero-overhead hardware capture without VRAM-to-RAM copies.
- **Smart Audio Router & Voice Isolation**:
  - Automatically isolates desktop and game audio.
  - Blacklists voice chat applications (**Discord, Vesktop, Slack, Zoom, Teams**) so voice call participants do not hear themselves echoing.
  - **Passive Mirror Mode**: Duplicates headphones directly with 0ms added delay and zero system modifications.
- **Modern Web Player**:
  - Custom HTML5 player powered by `mpegts.js`.
  - Built-in interactive **volume slider** (0–100%) with volume memory (`localStorage`).
  - One-click mute/unmute and dynamic speaker indicators (`🔊`, `🔉`, `🔈`, `🔇`).
  - Worker demuxing and tuned latency buffer management.
- **Global Public Access (Cloudflare Tunnel)**:
  - Accessible securely over public HTTPS (`wss://`) without exposing your home IP address or open router ports.
- **Test-Driven Architecture (TDD)**: 100% automated test coverage across shared utilities, client capture modules, server hubs, and end-to-end integration flows.

---

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    CLIENT PRODUCERS                     │
│                                                         │
│  [ Linux Wayland PC ] ──► Hardware VA-API ──┐          │
│                                             │ (HTTP     │
│  [ Windows / Linux PC ] ──► OBS Studio ─────┼── Stream  │
│                             (WHIP WebRTC)   │   Ingest) │
└─────────────────────────────────────────────┼───────────┘
                                              │
                                              ▼
┌─────────────────────────────────────────────────────────┐
│            SERVER (Distribution Hub / Ingest)           │
│                                                         │
│  Ingest Handlers:                                       │
│  - POST /api/publish   (Chunked MPEG-TS Ingest)         │
│  - POST /whip          (RFC WebRTC Ingestion for OBS)   │
│                                                         │
│  Broadcast Hubs:                                        │
│  - Go WebSocket Hub    (MPEG-TS fan-out to web players) │
│  - Pion WebRTC Hub     (Ultra-low latency RTP fan-out)  │
└─────────────────────────────┬───────────────────────────┘
                              │
               ┌──────────────┴──────────────┐
               ▼                             ▼
       [ Local Viewers ]           [ Cloudflare Tunnel ]
   (http://localhost:8080)                   │
                                             ▼
                                  [ Public Web Viewers ]
                             (https://stream.yourdomain.com)
```

---

## 🚀 Quick Start

### 1. Prerequisites

- **Linux** with **Wayland** (Hyprland, Sway, GNOME Wayland, KDE Plasma Wayland).
- **PipeWire** & **WirePlumber** (or `pipewire-media-session`).
- **Docker** & **Docker Compose**.
- (Optional) **Cloudflare Tunnel Token** for public sharing without IP exposure.

### 2. Configuration

Copy the example environment file:

```bash
cp .env.example .env
```

Edit `.env` to configure your settings:

```bash
# Cloudflare Tunnel Token (optional, for custom domain; otherwise quick tunnel is used)
CLOUDFLARE_TUNNEL_TOKEN=your_token_here

# Stream authentication key (optional; empty = open ingest)
STREAM_KEY=

# Audio capture mode:
# false = Passive headphone mirror (zero touch, recommended)
# true  = Voice isolation (filters Discord/Slack from stream)
AUDIO_ROUTING=false

# Video encoder mode (gpu = VA-API/NVENC, cpu = libx264)
ENCODER=gpu
FRAMERATE=60
VIDEO_BITRATE=6000k
```

---

### 3. Launching the Application

Use the unified [`./run`](file:///home/luigi/streaming/run) script:

```bash
# Launch the full stack (Distribution Server in Docker + local Wayland Client):
./run

# With specific options:
./run --gpu --mirror     # GPU hardware acceleration + passive headphone mirror
./run --cpu --isolate    # CPU software encoding + voice isolation
```

When prompted on your desktop, select the screen or window you want to share.

---

### 4. Running Specific Components

#### Run Only the Server (Ideal for OBS Studio or VPS Hosting)
```bash
./run server
```
* **Server URL**: `http://localhost:8080`
* **OBS WHIP Ingest**: `http://localhost:8080/whip`
* **Go Client Ingest**: `http://localhost:8080/api/publish`

#### Run Only the Capture Client
```bash
./run client --gpu --mirror
```
* Captures your desktop and pushes the stream to `http://localhost:8080/api/publish` (or a remote server set via `SERVER_URL`).

#### Legacy Monolithic Modes
```bash
./run legacy-ws        # Original monolithic WebSocket container
./run legacy-webrtc    # Original monolithic WebRTC container
```

---

## 🎥 Streaming from OBS Studio (Windows / Linux)

Because the server provides native **WHIP (WebRTC HTTP Ingestion Protocol)**, OBS Studio can stream directly to your server with **zero custom software required**:

1. Open **OBS Studio**.
2. Go to **Settings $\to$ Stream**:
   - **Service**: `WHIP`
   - **Server**: `http://<your-server-ip>:8080/whip` (or your Cloudflare Tunnel HTTPS URL)
   - **Bearer Token**: (Leave empty unless `STREAM_KEY` is configured)
3. Click **Start Streaming**.
4. Viewers open `http://<your-server-ip>:8080` (or your public tunnel domain) and watch instantly.

---

## 🔈 Audio Modes: Passive Mirror vs Voice Isolation

You can choose between two audio capture strategies:

### 1. Passive Headphone Mirror Mode (`--mirror` / `AUDIO_ROUTING=false`) [Recommended]
* **How it works**: Uses PipeWire's built-in sink monitor (`<default-sink>.monitor` or `easyeffects_sink.monitor`).
* **Zero System Touch**: Does **not** change your default audio device, does **not** move applications (Chrome, games, Spotify remain exactly where you put them), and does **not** interfere with EasyEffects.
* **Result**: Everything you hear in your headphones is mirrored into the live stream with 0ms added latency.

```bash
./run --gpu --mirror
```

### 2. Voice Isolation Mode (`--isolate` / `AUDIO_ROUTING=true`)
* **How it works**: Creates a virtual `stream_sink` and dynamically scans active audio streams:
  - **Games / YouTube / Chrome / Music** $\rightarrow$ Linked into `stream_sink` (streamed to viewers).
  - **Discord / Slack / Zoom Voice Calls** $\rightarrow$ Kept strictly on physical headphones (excluded from stream).

```bash
./run --gpu --isolate
```

---

## 🛠️ Project Structure

```
streaming/
│
├── client/                               # 📹 CLIENT (Producer / Collector)
│   ├── cmd/main.go                       # Client CLI entrypoint
│   ├── pkg/
│   │   ├── portal/                       # Wayland ScreenCast D-Bus client
│   │   ├── audio/                        # PulseAudio routing & blacklist filter
│   │   └── pipeline/                     # Zero-copy GStreamer pipeline & HTTP stream sender
│   └── Dockerfile                        # Client container definition
│
├── server/                               # 🌐 SERVER (Distribution Hub / Ingest)
│   ├── cmd/main.go                       # Server CLI entrypoint
│   ├── pkg/
│   │   ├── http/                         # HTTP router (/health, /ws, /whip, /api/publish)
│   │   ├── hub/                          # WebSocket MPEG-TS Hub & Pion WebRTC Hub
│   │   └── ingest/                       # HTTP chunked ingest & OBS WHIP ingest handlers
│   ├── web/                              # HTML5 low-latency web player (mpegts.js)
│   ├── Dockerfile                        # Lightweight pure-Go container (~25MB)
│   └── docker-compose.yml                # Server + Cloudflare Tunnel
│
├── utils/                                # 🛠️ SHARED UTILITIES
│   ├── config/                           # Environment variable parsers
│   ├── crypto/                           # Token generation & key validation
│   └── types/                            # StreamInfo & CaptureOptions data models
│
├── test/
│   └── integration/                      # Automated end-to-end integration test
│
├── docker-compose.yml                    # Root Docker Compose (Server + Tunnel + Client)
├── run                                   # Executable launcher
├── run.sh                                # Main launcher script
└── .env.example                          # Configuration template
```

---

## 🧪 Testing

Run the full automated unit and integration test suite:

```bash
go test -v ./...
```

---

## 📄 License

MIT License. Feel free to modify, host, and distribute.
