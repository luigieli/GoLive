package pipeline

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/luigieli/streaming/ws/pkg/server"
)

type Options struct {
	SourceWidth  int
	SourceHeight int
	TargetWidth  int
	TargetHeight int
	Framerate    int
	VideoBitrate int
	Encoder      string
	CPUThreads   int
	NodeID       int
	PipeWireFD   int
	AudioSource  string
}

type Runner struct {
	opts       Options
	hub        *server.Hub
	cmd        *exec.Cmd
	pipeReader *os.File
	pipeWriter *os.File
	mu         sync.Mutex
}

func NewRunner(opts Options, hub *server.Hub) *Runner {
	return &Runner{
		opts: opts,
		hub:  hub,
	}
}

func (r *Runner) buildGstArgs() []string {
	outWidth := r.opts.TargetWidth
	outHeight := r.opts.TargetHeight
	if outWidth <= 0 || outHeight <= 0 {
		outWidth = r.opts.SourceWidth
		outHeight = r.opts.SourceHeight
	}
	if outWidth <= 0 {
		outWidth = 1920
	}
	if outHeight <= 0 {
		outHeight = 1080
	}

	fps := r.opts.Framerate
	if fps <= 0 {
		fps = 30
	}

	threads := r.opts.CPUThreads
	if threads <= 0 {
		threads = 4
	}

	var encoderElements []string
	switch strings.ToLower(r.opts.Encoder) {
	case "cpu", "x264":
		encoderElements = []string{
			"!", fmt.Sprintf("video/x-raw,format=I420,width=%d,height=%d,framerate=%d/1", outWidth, outHeight, fps),
			"!", "x264enc",
			"tune=zerolatency",
			"speed-preset=ultrafast",
			fmt.Sprintf("bitrate=%d", r.opts.VideoBitrate),
			fmt.Sprintf("key-int-max=%d", fps),
			"bframes=0",
			fmt.Sprintf("threads=%d", threads),
			"sliced-threads=true",
			"rc-lookahead=0",
			"sync-lookahead=0",
			"byte-stream=true",
			"!", "video/x-h264,profile=constrained-baseline,stream-format=byte-stream",
		}
	case "nvenc":
		encoderElements = []string{
			"!", fmt.Sprintf("video/x-raw,format=NV12,width=%d,height=%d,framerate=%d/1", outWidth, outHeight, fps),
			"!", "nvh264enc",
			fmt.Sprintf("bitrate=%d", r.opts.VideoBitrate),
			fmt.Sprintf("gop-size=%d", fps),
			"rc-mode=cbr-ld-hq",
			"zerolatency=true",
			"!", "video/x-h264,profile=constrained-baseline,stream-format=byte-stream",
		}
	default: // "gpu", "vaapi", "auto"
		encoderElements = []string{
			"!", fmt.Sprintf("video/x-raw,format=NV12,width=%d,height=%d,framerate=%d/1", outWidth, outHeight, fps),
			"!", "vaapih264enc",
			"rate-control=cbr",
			fmt.Sprintf("bitrate=%d", r.opts.VideoBitrate),
			fmt.Sprintf("keyframe-period=%d", fps),
			"max-bframes=0",
			"tune=none",
			"!", "video/x-h264,profile=constrained-baseline,stream-format=byte-stream",
		}
	}

	args := []string{
		"-q",
		// Video Branch
		"pipewiresrc",
		"fd=3",
		fmt.Sprintf("path=%d", r.opts.NodeID),
		"do-timestamp=true",
		"keepalive-time=16",
		"always-copy=true",
		"!", "queue", "max-size-buffers=3", "max-size-time=0", "max-size-bytes=0", "leaky=downstream",
		"!", "videoconvert",
		"!", "videoscale", "method=1",
		"!", "videorate", "drop-only=false", "skip-to-first=true",
	}

	args = append(args, encoderElements...)
	args = append(args,
		"!", "h264parse",
		"!", "queue", "max-size-buffers=5", "leaky=downstream",
		"!", "mux.",

		// Audio Mixer with Silence Fallback (Prevents Muxer Deadlock)
		"audiomixer", "name=amix",
		"!", "audioconvert",
		"!", "audioresample",
		"!", "avenc_aac", "bitrate=128000",
		"!", "aacparse",
		"!", "queue", "max-size-buffers=5", "leaky=downstream",
		"!", "mux.",

		// Silence Clock Source
		"audiotestsrc", "is-live=true", "wave=silence", "volume=0.0",
		"!", "audio/x-raw,format=S16LE,rate=48000,channels=2",
		"!", "queue", "max-size-buffers=3", "leaky=downstream",
		"!", "amix.",

		// PulseAudio Source
		"pulsesrc",
		fmt.Sprintf("device=%s", r.opts.AudioSource),
		"do-timestamp=true",
		"!", "audio/x-raw,format=S16LE,rate=48000,channels=2",
		"!", "queue", "max-size-buffers=5", "leaky=downstream",
		"!", "amix.",

		// MPEG-TS Muxer
		"mpegtsmux", "name=mux", "alignment=7",
		"!", "fdsink", "fd=1", "sync=false",
	)

	return args
}

func (r *Runner) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	pr, pw, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("failed to create pipe: %w", err)
	}
	r.pipeReader = pr
	r.pipeWriter = pw

	args := r.buildGstArgs()
	r.cmd = exec.CommandContext(ctx, "gst-launch-1.0", args...)

	pipewireFile := os.NewFile(uintptr(r.opts.PipeWireFD), "pipewire-fd")
	r.cmd.ExtraFiles = []*os.File{pipewireFile}
	r.cmd.Stdout = r.pipeWriter
	r.cmd.Stderr = os.Stderr

	if err := r.cmd.Start(); err != nil {
		r.pipeWriter.Close()
		r.pipeReader.Close()
		return fmt.Errorf("failed to start gstreamer: %w", err)
	}

	go r.readLoop(ctx)

	go func() {
		_ = r.cmd.Wait()
		r.mu.Lock()
		if r.pipeWriter != nil {
			_ = r.pipeWriter.Close()
		}
		if r.pipeReader != nil {
			_ = r.pipeReader.Close()
		}
		r.mu.Unlock()
	}()

	return nil
}

func (r *Runner) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cmd != nil && r.cmd.Process != nil {
		_ = r.cmd.Process.Kill()
	}
	if r.pipeWriter != nil {
		_ = r.pipeWriter.Close()
	}
	if r.pipeReader != nil {
		_ = r.pipeReader.Close()
	}
}

func (r *Runner) readLoop(ctx context.Context) {
	const packetSize = 188
	const batchPackets = 348 // 65,424 bytes per chunk
	targetSize := packetSize * batchPackets

	buf := make([]byte, targetSize*2)
	offset := 0

	for {
		select {
		case <-ctx.Done():
			return
		default:
			n, err := r.pipeReader.Read(buf[offset:])
			if err != nil {
				return
			}
			offset += n

			completePackets := offset / packetSize
			if completePackets > 0 {
				bytesToSend := completePackets * packetSize
				if r.hub != nil {
					r.hub.Broadcast(buf[:bytesToSend])
				}
				remainder := offset - bytesToSend
				if remainder > 0 {
					copy(buf, buf[bytesToSend:offset])
				}
				offset = remainder
			}
		}
	}
}
