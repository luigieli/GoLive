package hub

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

func TestNewWebRTCHub(t *testing.T) {
	b, err := NewWebRTCHub([]string{"stun:stun.l.google.com:19302"}, nil)
	if err != nil {
		t.Fatalf("failed to create broadcaster: %v", err)
	}
	if b.VideoTrack() == nil {
		t.Errorf("expected VideoTrack to not be nil")
	}
	if b.AudioTrack() == nil {
		t.Errorf("expected AudioTrack to not be nil")
	}
}

func TestWebRTCHandleOffer(t *testing.T) {
	b, err := NewWebRTCHub([]string{"stun:stun.l.google.com:19302"}, nil)
	if err != nil {
		t.Fatalf("failed to create broadcaster: %v", err)
	}

	clientAPI := webrtc.NewAPI()
	clientPC, err := clientAPI.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatalf("failed to create client PC: %v", err)
	}
	defer clientPC.Close()

	_, err = clientPC.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionRecvonly,
	})
	if err != nil {
		t.Fatalf("failed to add video transceiver: %v", err)
	}

	_, err = clientPC.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionRecvonly,
	})
	if err != nil {
		t.Fatalf("failed to add audio transceiver: %v", err)
	}

	offer, err := clientPC.CreateOffer(nil)
	if err != nil {
		t.Fatalf("failed to create offer: %v", err)
	}

	answer, err := b.HandleOffer(offer)
	if err != nil {
		t.Fatalf("HandleOffer failed: %v", err)
	}

	if answer == nil || answer.SDP == "" {
		t.Errorf("expected valid answer with non-empty SDP")
	}
}

func TestWebRTCWriteRTP(t *testing.T) {
	b, err := NewWebRTCHub([]string{"stun:stun.l.google.com:19302"}, nil)
	if err != nil {
		t.Fatalf("failed to create broadcaster: %v", err)
	}

	pkt := &rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			PayloadType:    96,
			SequenceNumber: 1,
			Timestamp:      1000,
			SSRC:           12345,
		},
		Payload: []byte{0x67, 0x42, 0x00, 0x1f}, // dummy H.264
	}

	if err := b.WriteVideoRTP(pkt); err != nil {
		t.Errorf("failed to write video RTP: %v", err)
	}

	audioPkt := &rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			PayloadType:    111,
			SequenceNumber: 1,
			Timestamp:      2000,
			SSRC:           67890,
		},
		Payload: []byte{0x78, 0x01}, // dummy Opus
	}

	if err := b.WriteAudioRTP(audioPkt); err != nil {
		t.Errorf("failed to write audio RTP: %v", err)
	}
}
