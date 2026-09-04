package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/websocket"
	"github.com/luigieli/streaming/server/pkg/hub"
	"github.com/luigieli/streaming/server/pkg/ingest"
	"github.com/pion/webrtc/v4"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 65536,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Server struct {
	port       int
	wsHub      *hub.WSHub
	rtcHub     *hub.WebRTCHub
	streamKey  string
	webDir     string
	httpServer *http.Server
	mux        *http.ServeMux
}

func NewServer(port int, wsHub *hub.WSHub, rtcHub *hub.WebRTCHub, httpIngest *ingest.HTTPHandler, webDir string) *Server {
	s := &Server{
		port:   port,
		wsHub:  wsHub,
		rtcHub: rtcHub,
		webDir: webDir,
		mux:    http.NewServeMux(),
	}
	s.setupRoutes(httpIngest)
	return s
}

func (s *Server) setupRoutes(httpIngest *ingest.HTTPHandler) {
	s.mux.HandleFunc("/health", s.handleHealth)

	// WebSocket Viewer
	if s.wsHub != nil {
		s.mux.HandleFunc("/ws", s.handleWS)
	}

	// WebRTC Viewer Signaling
	if s.rtcHub != nil {
		s.mux.HandleFunc("/api/webrtc/offer", s.handleWebRTCOffer)
		// WHIP Ingest (for OBS)
		whipHandler := ingest.NewWHIPHandler(s.rtcHub, s.streamKey)
		s.mux.Handle("/whip", whipHandler)
	}

	// HTTP Ingest (for Go client)
	if httpIngest != nil {
		s.mux.Handle("/api/publish", httpIngest)
	}

	// Static Web Assets
	if s.webDir != "" {
		fs := http.FileServer(http.Dir(s.webDir))
		s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			path := filepath.Join(s.webDir, r.URL.Path)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				http.ServeFile(w, r, filepath.Join(s.webDir, "index.html"))
				return
			}
			fs.ServeHTTP(w, r)
		})
	}
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := hub.NewWSClient(s.wsHub, conn)
	s.wsHub.Register(client)

	go client.WritePump()
	go client.ReadPump()
}

func (s *Server) handleWebRTCOffer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var offer webrtc.SessionDescription
	if err := json.NewDecoder(r.Body).Decode(&offer); err != nil {
		http.Error(w, "Invalid SDP offer JSON", http.StatusBadRequest)
		return
	}

	answer, err := s.rtcHub.HandleOffer(offer)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to handle offer: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(answer)
}

func (s *Server) Start() error {
	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      s.mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	return s.httpServer.ListenAndServe()
}

func (s *Server) Stop(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}
