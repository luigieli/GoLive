package ingest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/luigieli/streaming/server/pkg/hub"
	"github.com/pion/webrtc/v4"
)

func TestWHIPIngestAuthorized(t *testing.T) {
	rtcHub, err := hub.NewWebRTCHub([]string{"stun:stun.l.google.com:19302"}, nil)
	if err != nil {
		t.Fatalf("failed to create rtc hub: %v", err)
	}

	handler := NewWHIPHandler(rtcHub, "secret-token")

	// Create client peer connection to simulate OBS WHIP client offer
	clientAPI := webrtc.NewAPI()
	clientPC, err := clientAPI.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatalf("failed to create client PC: %v", err)
	}
	defer clientPC.Close()

	_, _ = clientPC.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionSendonly,
	})
	_, _ = clientPC.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionSendonly,
	})

	offer, err := clientPC.CreateOffer(nil)
	if err != nil {
		t.Fatalf("failed to create offer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/whip", strings.NewReader(offer.SDP))
	req.Header.Set("Content-Type", "application/sdp")
	req.Header.Set("Authorization", "Bearer secret-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201 Created for WHIP, got %d: %s", w.Code, w.Body.String())
	}

	if w.Header().Get("Content-Type") != "application/sdp" {
		t.Errorf("expected Content-Type application/sdp, got %s", w.Header().Get("Content-Type"))
	}

	answerSDP := w.Body.String()
	if !strings.Contains(answerSDP, "v=0") {
		t.Errorf("expected valid SDP answer in response body, got: %s", answerSDP)
	}
}

func TestWHIPUnauthorized(t *testing.T) {
	rtcHub, err := hub.NewWebRTCHub([]string{"stun:stun.l.google.com:19302"}, nil)
	if err != nil {
		t.Fatalf("failed to create rtc hub: %v", err)
	}

	handler := NewWHIPHandler(rtcHub, "secret-token")

	req := httptest.NewRequest(http.MethodPost, "/whip", strings.NewReader("dummy sdp"))
	req.Header.Set("Content-Type", "application/sdp")
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 Unauthorized for WHIP, got %d", w.Code)
	}
}
