package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Live Screen Stream (HLS)</title>
    <script src="https://cdn.jsdelivr.net/npm/hls.js@latest"></script>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            background-color: #0f172a;
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
            color: #f8fafc;
            display: flex;
            flex-direction: column;
            align-items: center;
            justify-content: center;
            min-height: 100vh;
            padding: 20px;
        }
        .container {
            width: 100%;
            max-width: 1280px;
            background-color: #1e293b;
            border-radius: 12px;
            overflow: hidden;
            box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.5), 0 8px 10px -6px rgba(0, 0, 0, 0.5);
            border: 1px solid #334155;
        }
        .header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 16px 24px;
            background-color: #0f172a;
            border-bottom: 1px solid #334155;
        }
        .title-group { display: flex; align-items: center; gap: 12px; }
        h1 { font-size: 1.25rem; font-weight: 600; }
        .live-badge {
            display: inline-flex;
            align-items: center;
            gap: 6px;
            background-color: #ef4444;
            color: white;
            padding: 3px 10px;
            border-radius: 9999px;
            font-size: 0.75rem;
            font-weight: 700;
            letter-spacing: 0.05em;
            text-transform: uppercase;
        }
        .live-dot {
            width: 8px;
            height: 8px;
            background-color: white;
            border-radius: 50%;
            animation: pulse 1.5s infinite;
        }
        @keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.3; } }
        .status-badge { font-size: 0.85rem; color: #94a3b8; }
        .video-wrapper {
            position: relative;
            width: 100%;
            background-color: #000;
            aspect-ratio: 16 / 9;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        video { width: 100%; height: 100%; object-fit: contain; }
        .footer-info {
            padding: 16px 24px;
            display: flex;
            justify-content: space-between;
            align-items: center;
            font-size: 0.875rem;
            color: #94a3b8;
        }
        .info-pill {
            background-color: #0f172a;
            padding: 4px 12px;
            border-radius: 6px;
            border: 1px solid #334155;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <div class="title-group">
                <h1>Desktop Live Stream</h1>
                <span class="live-badge" id="liveBadge">
                    <span class="live-dot"></span> LIVE
                </span>
            </div>
            <div class="status-badge" id="streamStatus">Connecting to stream...</div>
        </div>

        <div class="video-wrapper">
            <video id="video" controls autoplay muted playsinline></video>
        </div>

        <div class="footer-info">
            <div>Format: <strong>HLS (H.264 / AAC)</strong></div>
            <div class="info-pill" id="statsPill">Native Resolution Stream Active</div>
        </div>
    </div>

    <script>
        const video = document.getElementById('video');
        const statusEl = document.getElementById('streamStatus');
        const streamUrl = '/hls/index.m3u8';

        if (Hls.isSupported()) {
            const hls = new Hls({
                liveSyncDurationCount: 3,
                liveMaxLatencyDurationCount: 8,
                enableWorker: true,
                lowLatencyMode: true,
                backBufferLength: 30,
                maxBufferLength: 30,
                maxMaxBufferLength: 60,
                manifestLoadingMaxRetry: Infinity,
                manifestLoadingRetryDelay: 1000,
                levelLoadingMaxRetry: Infinity,
                levelLoadingRetryDelay: 1000,
                fragLoadingMaxRetry: Infinity,
                fragLoadingRetryDelay: 1000
            });

            hls.loadSource(streamUrl);
            hls.attachMedia(video);

            hls.on(Hls.Events.MANIFEST_PARSED, () => {
                statusEl.textContent = 'Stream connected';
                video.play().catch(e => console.log('Autoplay:', e));
            });

            hls.on(Hls.Events.ERROR, (event, data) => {
                if (data.fatal) {
                    statusEl.textContent = 'Waiting for live feed...';
                    switch (data.type) {
                        case Hls.ErrorTypes.NETWORK_ERROR:
                            hls.startLoad();
                            break;
                        case Hls.ErrorTypes.MEDIA_ERROR:
                            hls.recoverMediaError();
                            break;
                        default:
                            hls.destroy();
                            break;
                    }
                }
            });
        } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
            video.src = streamUrl;
            video.addEventListener('loadedmetadata', () => {
                video.play();
                statusEl.textContent = 'Stream connected';
            });
        }
    </script>
</body>
</html>`

type Server struct {
	hlsDir string
	mux    *http.ServeMux
}

func NewServer(hlsDir string) *Server {
	s := &Server{
		hlsDir: hlsDir,
		mux:    http.NewServeMux(),
	}
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	s.mux.HandleFunc("/", s.handleRoot)
	s.mux.HandleFunc("/hls/", s.handleHLS)
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, max-age=0, s-maxage=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(indexHTML))
}

func (s *Server) handleHLS(w http.ResponseWriter, r *http.Request) {
	// Add global CORS headers for playback
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Range")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	cleanPath := strings.TrimPrefix(r.URL.Path, "/hls/")
	if cleanPath == "" || strings.Contains(cleanPath, "..") {
		http.NotFound(w, r)
		return
	}

	filePath := filepath.Join(s.hlsDir, cleanPath)
	data, err := os.ReadFile(filePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if strings.HasSuffix(cleanPath, ".m3u8") {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, max-age=0, s-maxage=0")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
	} else if strings.HasSuffix(cleanPath, ".ts") {
		w.Header().Set("Content-Type", "video/mp2t")
		w.Header().Set("Cache-Control", "public, max-age=60")
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) ListenAndServe(port int) error {
	addr := fmt.Sprintf(":%d", port)
	return http.ListenAndServe(addr, s.mux)
}
