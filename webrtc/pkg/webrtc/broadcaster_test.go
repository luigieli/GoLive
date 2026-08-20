package webrtc

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

func TestNewBroadcaster(t *testing.T) {
	b, err := NewBroadcaster([]string{"stun:stun.l.google.com:19302"})
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

func TestHandleOffer(t *testing.T) {
	b, err := NewBroadcaster([]string{"stun:stun.l.google.com:19302"})
	if err != nil {
		t.Fatalf("failed to create broadcaster: %v", err)
	}

	// Create client peer connection to generate valid SDP offer
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
		t.Errorf("expected valid answer SDP, got empty")
	}
	if answer.Type != webrtc.SDPTypeAnswer {
		t.Errorf("expected answer type 'answer', got %v", answer.Type)
	}
}

func TestWriteRTP(t *testing.T) {
	b, err := NewBroadcaster([]string{"stun:stun.l.google.com:19302"})
	if err != nil {
		t.Fatalf("failed to create broadcaster: %v", err)
	}

	videoPkt := &rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			PayloadType:    96,
			SequenceNumber: 1,
			Timestamp:      1000,
			SSRC:           12345,
		},
		Payload: []byte{0x67, 0x42, 0x00, 0x1f}, // H.264 SPS dummy
	}

	if err := b.WriteVideoRTP(videoPkt); err != nil {
		t.Errorf("WriteVideoRTP error: %v", err)
	}

	audioPkt := &rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			PayloadType:    111,
			SequenceNumber: 1,
			Timestamp:      960,
			SSRC:           54321,
		},
		Payload: []byte{0xf8, 0xff, 0xfe}, // Opus dummy
	}

	if err := b.WriteAudioRTP(audioPkt); err != nil {
		t.Errorf("WriteAudioRTP error: %v", err)
	}
}
