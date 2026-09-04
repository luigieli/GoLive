package ingest

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/luigieli/streaming/server/pkg/hub"
	"github.com/luigieli/streaming/utils/crypto"
	"github.com/pion/webrtc/v4"
)

type WHIPHandler struct {
	hub       *hub.WebRTCHub
	streamKey string
}

func NewWHIPHandler(h *hub.WebRTCHub, streamKey string) *WHIPHandler {
	return &WHIPHandler{
		hub:       h,
		streamKey: streamKey,
	}
}

func (h *WHIPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Authenticate bearer token or query key
	key := r.URL.Query().Get("key")
	if key == "" {
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			key = strings.TrimPrefix(auth, "Bearer ")
		}
	}

	if !crypto.ValidateKey(h.streamKey, key) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read SDP offer", http.StatusBadRequest)
		return
	}
	offerSDP := string(bodyBytes)
	if strings.TrimSpace(offerSDP) == "" {
		http.Error(w, "Empty SDP offer", http.StatusBadRequest)
		return
	}

	// Create MediaEngine for receiving incoming ingest tracks
	mediaEngine := &webrtc.MediaEngine{}
	_ = mediaEngine.RegisterDefaultCodecs()

	api := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine))
	pc, err := api.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create PeerConnection: %v", err), http.StatusInternalServerError)
		return
	}

	// Hook incoming tracks from OBS/publisher
	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		if track.Kind() == webrtc.RTPCodecTypeVideo {
			go func() {
				for {
					pkt, _, readErr := track.ReadRTP()
					if readErr != nil {
						return
					}
					_ = h.hub.WriteVideoRTP(pkt)
				}
			}()
		} else if track.Kind() == webrtc.RTPCodecTypeAudio {
			go func() {
				for {
					pkt, _, readErr := track.ReadRTP()
					if readErr != nil {
						return
					}
					_ = h.hub.WriteAudioRTP(pkt)
				}
			}()
		}
	})

	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  offerSDP,
	}

	if err := pc.SetRemoteDescription(offer); err != nil {
		_ = pc.Close()
		http.Error(w, fmt.Sprintf("Failed to set remote description: %v", err), http.StatusBadRequest)
		return
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		_ = pc.Close()
		http.Error(w, fmt.Sprintf("Failed to create answer: %v", err), http.StatusInternalServerError)
		return
	}

	gatherComplete := webrtc.GatheringCompletePromise(pc)
	if err := pc.SetLocalDescription(answer); err != nil {
		_ = pc.Close()
		http.Error(w, fmt.Sprintf("Failed to set local description: %v", err), http.StatusInternalServerError)
		return
	}

	<-gatherComplete

	w.Header().Set("Content-Type", "application/sdp")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write([]byte(pc.LocalDescription().SDP))
}
