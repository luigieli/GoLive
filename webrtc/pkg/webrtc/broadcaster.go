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

func NewBroadcaster(iceServers []string, natIPs ...[]string) (*Broadcaster, error) {
	var iceList []webrtc.ICEServer
	for _, s := range iceServers {
		if s != "" {
			iceList = append(iceList, webrtc.ICEServer{URLs: []string{s}})
		}
	}

	mediaEngine := &webrtc.MediaEngine{}

	// Register H.264 as primary video codec with payload type 96 (constrained-baseline)
	if err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:     webrtc.MimeTypeH264,
			ClockRate:    90000,
			Channels:     0,
			SDPFmtpLine:  "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f",
			RTCPFeedback: []webrtc.RTCPFeedback{{Type: "nack"}, {Type: "nack", Parameter: "pli"}, {Type: "ccm", Parameter: "fir"}, {Type: "goog-remb"}},
		},
		PayloadType: 96,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		return nil, fmt.Errorf("failed to register h264 codec: %w", err)
	}

	// Register H.264 High profile as compatible alternative (payload type 98)
	_ = mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:     webrtc.MimeTypeH264,
			ClockRate:    90000,
			Channels:     0,
			SDPFmtpLine:  "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=64001f",
			RTCPFeedback: []webrtc.RTCPFeedback{{Type: "nack"}, {Type: "nack", Parameter: "pli"}, {Type: "ccm", Parameter: "fir"}, {Type: "goog-remb"}},
		},
		PayloadType: 98,
	}, webrtc.RTPCodecTypeVideo)

	// Register Opus as primary audio codec with payload type 111
	if err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:     webrtc.MimeTypeOpus,
			ClockRate:    48000,
			Channels:     2,
			SDPFmtpLine:  "minptime=10;useinbandfec=1",
			RTCPFeedback: nil,
		},
		PayloadType: 111,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		return nil, fmt.Errorf("failed to register opus codec: %w", err)
	}

	settingEngine := webrtc.SettingEngine{}
	_ = settingEngine.SetEphemeralUDPPortRange(50000, 50050)
	if len(natIPs) > 0 && len(natIPs[0]) > 0 {
		var validIPs []string
		for _, ip := range natIPs[0] {
			if ip != "" {
				validIPs = append(validIPs, ip)
			}
		}
		if len(validIPs) > 0 {
			settingEngine.SetNAT1To1IPs(validIPs, webrtc.ICECandidateTypeHost)
		}
	}

	api := webrtc.NewAPI(
		webrtc.WithMediaEngine(mediaEngine),
		webrtc.WithSettingEngine(settingEngine),
	)

	videoTrack, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{
			MimeType:    webrtc.MimeTypeH264,
			ClockRate:   90000,
			SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f",
		},
		"video",
		"webrtc-stream",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create video track: %w", err)
	}

	audioTrack, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{
			MimeType:  webrtc.MimeTypeOpus,
			ClockRate: 48000,
			Channels:  2,
		},
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

	// Add tracks to PeerConnection & read RTCP feedback in background
	videoSender, err := pc.AddTrack(b.videoTrack)
	if err != nil {
		pc.Close()
		return nil, fmt.Errorf("failed to add video track: %w", err)
	}
	go func() {
		buf := make([]byte, 1500)
		for {
			if _, _, err := videoSender.Read(buf); err != nil {
				return
			}
		}
	}()

	audioSender, err := pc.AddTrack(b.audioTrack)
	if err != nil {
		pc.Close()
		return nil, fmt.Errorf("failed to add audio track: %w", err)
	}
	go func() {
		buf := make([]byte, 1500)
		for {
			if _, _, err := audioSender.Read(buf); err != nil {
				return
			}
		}
	}()

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
	pkt.Header.PayloadType = 96
	return b.videoTrack.WriteRTP(pkt)
}

func (b *Broadcaster) WriteAudioRTP(pkt *rtp.Packet) error {
	pkt.Header.PayloadType = 111
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
