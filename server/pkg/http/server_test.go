package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/luigieli/streaming/server/pkg/hub"
	"github.com/pion/webrtc/v4"
)

func TestServerHealth(t *testing.T) {
	wsHub := hub.NewWSHub()
	s := NewServer(8080, wsHub, nil, nil, "")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	s.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %s", body["status"])
	}
}

func TestServerWebRTCOffer(t *testing.T) {
	wsHub := hub.NewWSHub()
	rtcHub, err := hub.NewWebRTCHub([]string{"stun:stun.l.google.com:19302"}, nil)
	if err != nil {
		t.Fatalf("failed to create rtc hub: %v", err)
	}

	s := NewServer(8080, wsHub, rtcHub, nil, "")

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

	offer, err := clientPC.CreateOffer(nil)
	if err != nil {
		t.Fatalf("failed to create offer: %v", err)
	}

	offerJSON, _ := json.Marshal(offer)
	req := httptest.NewRequest(http.MethodPost, "/api/webrtc/offer", bytes.NewReader(offerJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var answer webrtc.SessionDescription
	if err := json.NewDecoder(w.Body).Decode(&answer); err != nil {
		t.Fatalf("failed to decode answer: %v", err)
	}
	if answer.SDP == "" {
		t.Errorf("expected non-empty SDP answer")
	}
}
