package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/pion/webrtc/v4"
	webrtcPkg "github.com/luigieli/streaming/webrtc/pkg/webrtc"
)

func TestServerHealth(t *testing.T) {
	srv := NewServer(".", nil)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", rec.Code)
	}
}

func TestServerRoot(t *testing.T) {
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "index.html")
	_ = os.WriteFile(indexPath, []byte("<h1>WebRTC Player</h1>"), 0644)

	srv := NewServer(tmpDir, nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("WebRTC Player")) {
		t.Errorf("expected body to contain WebRTC Player, got: %s", rec.Body.String())
	}
}

func TestServerOffer(t *testing.T) {
	b, err := webrtcPkg.NewBroadcaster([]string{"stun:stun.l.google.com:19302"})
	if err != nil {
		t.Fatalf("failed to create broadcaster: %v", err)
	}

	clientAPI := webrtc.NewAPI()
	clientPC, err := clientAPI.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatalf("failed to create client PC: %v", err)
	}
	defer clientPC.Close()

	_, _ = clientPC.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionRecvonly,
	})
	_, _ = clientPC.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionRecvonly,
	})

	offer, _ := clientPC.CreateOffer(nil)
	offerJSON, _ := json.Marshal(offer)

	srv := NewServer(".", b)
	req := httptest.NewRequest(http.MethodPost, "/api/webrtc/offer", bytes.NewReader(offerJSON))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var answer webrtc.SessionDescription
	if err := json.Unmarshal(rec.Body.Bytes(), &answer); err != nil {
		t.Fatalf("failed to parse answer JSON: %v", err)
	}
	if answer.Type != webrtc.SDPTypeAnswer || answer.SDP == "" {
		t.Errorf("invalid answer SDP returned: %v", answer)
	}
}
