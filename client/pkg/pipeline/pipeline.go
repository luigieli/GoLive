package pipeline

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/luigieli/streaming/utils/types"
)

type Options = types.CaptureOptions

type Runner struct {
	opts       Options
	sender     *Sender
	cmd        *exec.Cmd
	pipeReader *os.File
	pipeWriter *os.File
	cancelFunc context.CancelFunc
	mu         sync.Mutex
}

func NewRunner(opts Options, sender *Sender) *Runner {
	return &Runner{
		opts:   opts,
		sender: sender,
	}
}

func (r *Runner) buildGstArgs() []string {
	width := r.opts.Width
	if width <= 0 {
		width = 1920
	}
	height := r.opts.Height
	if height <= 0 {
		height = 1080
	}
	fps := r.opts.Framerate
	if fps <= 0 {
		fps = 60
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
			"!", fmt.Sprintf("video/x-raw,format=I420,width=%d,height=%d,framerate=%d/1", width, height, fps),
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
			"!", fmt.Sprintf("video/x-raw,format=NV12,width=%d,height=%d,framerate=%d/1", width, height, fps),
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
			"!", "videorate",
			"!", fmt.Sprintf("video/x-raw(ANY),framerate=%d/1", fps),
			"!", "vaapipostproc", "scale-method=2", "format=nv12",
			fmt.Sprintf("width=%d", width),
			fmt.Sprintf("height=%d", height),
			"!", fmt.Sprintf("video/x-raw(memory:VASurface),width=%d,height=%d,framerate=%d/1", width, height, fps),
			"!", "vaapih264enc",
			"aud=true",
			"rate-control=cbr",
			"cabac=true",
			"dct8x8=true",
			"quality-level=1",
			"cpb-length=1000",
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
		"always-copy=false",
		"!", "queue", "max-size-buffers=15", "max-size-time=200000000", "max-size-bytes=0", "leaky=downstream",
	}

	args = append(args, encoderElements...)
	args = append(args,
		// Audio Branch
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

func (r *Runner) Start(ctx context.Context) (io.ReadCloser, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	ctx, cancel := context.WithCancel(ctx)
	r.cancelFunc = cancel

	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create pipe: %w", err)
	}
	r.pipeReader = pr
	r.pipeWriter = pw

	args := r.buildGstArgs()
	r.cmd = exec.CommandContext(ctx, "gst-launch-1.0", args...)
	r.cmd.ExtraFiles = []*os.File{os.NewFile(uintptr(r.opts.PipeWireFD), "pipewire-fd")}
	r.cmd.Stdout = r.pipeWriter
	r.cmd.Stderr = os.Stderr

	if err := r.cmd.Start(); err != nil {
		_ = r.pipeWriter.Close()
		_ = r.pipeReader.Close()
		return nil, fmt.Errorf("failed to start gstreamer: %w", err)
	}

	go func() {
		_ = r.cmd.Wait()
		_ = r.pipeWriter.Close()
	}()

	return r.pipeReader, nil
}

func (r *Runner) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cancelFunc != nil {
		r.cancelFunc()
	}
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
