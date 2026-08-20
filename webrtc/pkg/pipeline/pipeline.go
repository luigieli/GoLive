package pipeline

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sync"

	"github.com/pion/rtp"
	webrtcPkg "github.com/luigieli/streaming/webrtc/pkg/webrtc"
)

type Options struct {
	Width        int
	Height       int
	Framerate    int
	VideoBitrate int
	NodeID       int
	PipeWireFD   int
	AudioSource  string
	VideoUDPPort int
	AudioUDPPort int
}

type Runner struct {
	opts        Options
	broadcaster *webrtcPkg.Broadcaster
	gstCmd      *exec.Cmd
	videoConn   *net.UDPConn
	audioConn   *net.UDPConn
	cancelFunc  context.CancelFunc
	mu          sync.Mutex
}

func NewRunner(opts Options, broadcaster *webrtcPkg.Broadcaster) *Runner {
	if opts.VideoUDPPort <= 0 {
		opts.VideoUDPPort = 5004
	}
	if opts.AudioUDPPort <= 0 {
		opts.AudioUDPPort = 5006
	}
	return &Runner{
		opts:        opts,
		broadcaster: broadcaster,
	}
}

func (r *Runner) buildGstArgs() []string {
	return []string{
		"-q",
		// Video Pipeline
		"pipewiresrc",
		"fd=3",
		fmt.Sprintf("path=%d", r.opts.NodeID),
		"do-timestamp=true",
		"keepalive-time=33",
		"!", "queue", "max-size-buffers=3", "leaky=downstream",
		"!", "videoconvert",
		"!", "videoscale",
		"!", "videorate", "drop-only=false", "skip-to-first=true",
		"!", fmt.Sprintf("video/x-raw,format=I420,width=%d,height=%d,framerate=%d/1", r.opts.Width, r.opts.Height, r.opts.Framerate),
		"!", "x264enc",
		"tune=zerolatency",
		"speed-preset=ultrafast",
		fmt.Sprintf("bitrate=%d", r.opts.VideoBitrate),
		"key-int-max=30",
		"bframes=0",
		"sliced-threads=true",
		"byte-stream=true",
		"!", "rtph264pay",
		"config-interval=1",
		"pt=96",
		"aggregate-mode=zero-latency",
		"!", "udpsink",
		"host=127.0.0.1",
		fmt.Sprintf("port=%d", r.opts.VideoUDPPort),
		"sync=false",

		// Audio Pipeline
		"pulsesrc",
		fmt.Sprintf("device=%s", r.opts.AudioSource),
		"!", "audioconvert",
		"!", "audioresample",
		"!", "opusenc",
		"bitrate=128000",
		"frame-size=20",
		"!", "rtpopuspay",
		"pt=111",
		"!", "udpsink",
		"host=127.0.0.1",
		fmt.Sprintf("port=%d", r.opts.AudioUDPPort),
		"sync=false",
	}
}

func (r *Runner) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	ctx, cancel := context.WithCancel(ctx)
	r.cancelFunc = cancel

	// 1. Setup UDP Listeners in Go
	videoAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", r.opts.VideoUDPPort))
	if err != nil {
		return fmt.Errorf("failed to resolve video UDP addr: %w", err)
	}
	videoConn, err := net.ListenUDP("udp", videoAddr)
	if err != nil {
		return fmt.Errorf("failed to listen video UDP: %w", err)
	}
	_ = videoConn.SetReadBuffer(4 * 1024 * 1024)
	r.videoConn = videoConn

	audioAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", r.opts.AudioUDPPort))
	if err != nil {
		videoConn.Close()
		return fmt.Errorf("failed to resolve audio UDP addr: %w", err)
	}
	audioConn, err := net.ListenUDP("udp", audioAddr)
	if err != nil {
		videoConn.Close()
		return fmt.Errorf("failed to listen audio UDP: %w", err)
	}
	_ = audioConn.SetReadBuffer(1 * 1024 * 1024)
	r.audioConn = audioConn

	// 2. Start Video and Audio Ingestion Loops
	go r.readVideoLoop(ctx)
	go r.readAudioLoop(ctx)

	// 3. Start GStreamer Process
	gstArgs := r.buildGstArgs()
	r.gstCmd = exec.CommandContext(ctx, "gst-launch-1.0", gstArgs...)
	r.gstCmd.ExtraFiles = []*os.File{os.NewFile(uintptr(r.opts.PipeWireFD), "pipewire-fd")}
	r.gstCmd.Stdout = os.Stdout
	r.gstCmd.Stderr = os.Stderr

	if err := r.gstCmd.Start(); err != nil {
		r.Stop()
		return fmt.Errorf("failed to start gstreamer webrtc pipeline: %w", err)
	}

	return nil
}

func (r *Runner) readVideoLoop(ctx context.Context) {
	buf := make([]byte, 65535)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			n, _, err := r.videoConn.ReadFrom(buf)
			if err != nil {
				return
			}
			if n > 0 && r.broadcaster != nil {
				pkt := &rtp.Packet{}
				if err := pkt.Unmarshal(buf[:n]); err == nil {
					_ = r.broadcaster.WriteVideoRTP(pkt)
				}
			}
		}
	}
}

func (r *Runner) readAudioLoop(ctx context.Context) {
	buf := make([]byte, 65535)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			n, _, err := r.audioConn.ReadFrom(buf)
			if err != nil {
				return
			}
			if n > 0 && r.broadcaster != nil {
				pkt := &rtp.Packet{}
				if err := pkt.Unmarshal(buf[:n]); err == nil {
					_ = r.broadcaster.WriteAudioRTP(pkt)
				}
			}
		}
	}
}

func (r *Runner) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cancelFunc != nil {
		r.cancelFunc()
	}
	if r.videoConn != nil {
		_ = r.videoConn.Close()
	}
	if r.audioConn != nil {
		_ = r.audioConn.Close()
	}
	if r.gstCmd != nil && r.gstCmd.Process != nil {
		_ = r.gstCmd.Process.Kill()
	}
}
