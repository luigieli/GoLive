package pipeline

import (
	"bufio"
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
			"!", "videoconvert",
			"!", "videoscale", "method=1",
			"!", "videorate",
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
			"!", "h264parse", "config-interval=-1",
			"!", "queue", "max-size-buffers=5", "leaky=downstream",
			"!", "mux.",
		}
	case "nvenc":
		encoderElements = []string{
			"!", "videoconvert",
			"!", "videoscale", "method=1",
			"!", "videorate",
			"!", fmt.Sprintf("video/x-raw,format=NV12,width=%d,height=%d,framerate=%d/1", outWidth, outHeight, fps),
			"!", "nvh264enc",
			fmt.Sprintf("bitrate=%d", r.opts.VideoBitrate),
			fmt.Sprintf("gop-size=%d", fps),
			"rc-mode=cbr-ld-hq",
			"zerolatency=true",
			"!", "video/x-h264,profile=constrained-baseline,stream-format=byte-stream",
			"!", "h264parse", "config-interval=-1",
			"!", "queue", "max-size-buffers=5", "leaky=downstream",
			"!", "mux.",
		}
	default: // "gpu", "vaapi", "auto"
		encoderElements = []string{
			"!", "videoconvert",
			"!", "videorate",
			"!", fmt.Sprintf("video/x-raw,framerate=%d/1", fps),
			"!", "vaapipostproc", "scale-method=2", "format=nv12",
			fmt.Sprintf("width=%d", outWidth),
			fmt.Sprintf("height=%d", outHeight),
			"!", fmt.Sprintf("video/x-raw(memory:VASurface),width=%d,height=%d,framerate=%d/1", outWidth, outHeight, fps),
			"!", "vaapih264enc",
			"aud=true",
			"rate-control=cbr",
			"cabac=true",
			"dct8x8=true",
			"quality-level=1",
			fmt.Sprintf("bitrate=%d", r.opts.VideoBitrate),
			fmt.Sprintf("keyframe-period=%d", fps),
			"max-bframes=0",
			"tune=none",
			"!", "video/x-h264,profile=high,stream-format=byte-stream",
			"!", "h264parse", "config-interval=1",
			"!", "queue", "max-size-buffers=30", "max-size-time=0", "max-size-bytes=0",
			"!", "mux.",
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
		"!", "video/x-raw",
		"!", "queue", "max-size-buffers=3", "max-size-time=0", "max-size-bytes=0", "leaky=downstream",
	}

	args = append(args, encoderElements...)
	args = append(args,

		// Direct PulseAudio Source -> AAC Audio Branch
		"pulsesrc",
		fmt.Sprintf("device=%s", r.opts.AudioSource),
		"do-timestamp=true",
		"provide-clock=false",
		"!", "audio/x-raw,format=S16LE,rate=48000,channels=2",
		"!", "queue", "max-size-buffers=30",
		"!", "audioconvert",
		"!", "audioresample",
		"!", "audiorate",
		"!", "avenc_aac", "bitrate=192000",
		"!", "aacparse",
		"!", "queue", "max-size-buffers=30", "max-size-time=0", "max-size-bytes=0",
		"!", "mux.",

		// MPEG-TS Muxer
		"mpegtsmux", "name=mux", "alignment=7", "pat-interval=4500", "pmt-interval=4500", "pcr-interval=1800",
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
	const readBufSize = 65536

	reader := bufio.NewReaderSize(r.pipeReader, 1048576)
	tsBuffer := make([]byte, 0, 131600)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			buf := make([]byte, readBufSize)
			n, err := reader.Read(buf)
			if err != nil {
				return
			}
			if n > 0 {
				tsBuffer = append(tsBuffer, buf[:n]...)
				completePackets := len(tsBuffer) / packetSize
				if completePackets > 0 {
					bytesToSend := completePackets * packetSize
					chunk := make([]byte, bytesToSend)
					copy(chunk, tsBuffer[:bytesToSend])

					if r.hub != nil {
						r.hub.Broadcast(chunk)
					}

					remainder := len(tsBuffer) - bytesToSend
					if remainder > 0 {
						newBuf := make([]byte, remainder, 131600)
						copy(newBuf, tsBuffer[bytesToSend:])
						tsBuffer = newBuf
					} else {
						tsBuffer = tsBuffer[:0]
					}
				}
			}
		}
	}
}
