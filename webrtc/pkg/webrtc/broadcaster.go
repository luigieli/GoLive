package webrtc

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

type Broadcaster struct {
	api             *webrtc.API
	config          webrtc.Configuration
	videoTrack      *webrtc.TrackLocalStaticRTP
	audioTrack      *webrtc.TrackLocalStaticRTP
	peerConnections map[string]*webrtc.PeerConnection
	mu              sync.RWMutex
}

func NewBroadcaster(iceServers []string) (*Broadcaster, error) {
	var iceList []webrtc.ICEServer
	for _, s := range iceServers {
		if s != "" {
			iceList = append(iceList, webrtc.ICEServer{URLs: []string{s}})
		}
	}

	mediaEngine := &webrtc.MediaEngine{}
	if err := mediaEngine.RegisterDefaultCodecs(); err != nil {
		return nil, fmt.Errorf("failed to register default codecs: %w", err)
	}

	api := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine))

	videoTrack, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264},
		"video",
		"webrtc-stream",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create video track: %w", err)
	}

	audioTrack, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus},
		"audio",
		"webrtc-stream",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create audio track: %w", err)
	}

	return &Broadcaster{
		api: api,
		config: webrtc.Configuration{
			ICEServers: iceList,
		},
		videoTrack:      videoTrack,
		audioTrack:      audioTrack,
		peerConnections: make(map[string]*webrtc.PeerConnection),
	}, nil
}

func (b *Broadcaster) VideoTrack() *webrtc.TrackLocalStaticRTP {
	return b.videoTrack
}

func (b *Broadcaster) AudioTrack() *webrtc.TrackLocalStaticRTP {
	return b.audioTrack
}

func (b *Broadcaster) HandleOffer(offer webrtc.SessionDescription) (*webrtc.SessionDescription, error) {
	pc, err := b.api.NewPeerConnection(b.config)
	if err != nil {
		return nil, fmt.Errorf("failed to create peer connection: %w", err)
	}

	peerID := uuid.New().String()
	b.mu.Lock()
	b.peerConnections[peerID] = pc
	b.mu.Unlock()

	// Add tracks to PeerConnection
	if _, err := pc.AddTrack(b.videoTrack); err != nil {
		pc.Close()
		return nil, fmt.Errorf("failed to add video track: %w", err)
	}
	if _, err := pc.AddTrack(b.audioTrack); err != nil {
		pc.Close()
		return nil, fmt.Errorf("failed to add audio track: %w", err)
	}

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateClosed {
			b.mu.Lock()
			delete(b.peerConnections, peerID)
			b.mu.Unlock()
			_ = pc.Close()
		}
	})

	if err := pc.SetRemoteDescription(offer); err != nil {
		pc.Close()
		return nil, fmt.Errorf("failed to set remote description: %w", err)
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		pc.Close()
		return nil, fmt.Errorf("failed to create answer: %w", err)
	}

	gatherComplete := webrtc.GatheringCompletePromise(pc)

	if err := pc.SetLocalDescription(answer); err != nil {
		pc.Close()
		return nil, fmt.Errorf("failed to set local description: %w", err)
	}

	<-gatherComplete

	return pc.LocalDescription(), nil
}

func (b *Broadcaster) WriteVideoRTP(pkt *rtp.Packet) error {
	return b.videoTrack.WriteRTP(pkt)
}

func (b *Broadcaster) WriteAudioRTP(pkt *rtp.Packet) error {
	return b.audioTrack.WriteRTP(pkt)
}

func (b *Broadcaster) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for id, pc := range b.peerConnections {
		_ = pc.Close()
		delete(b.peerConnections, id)
	}
}
