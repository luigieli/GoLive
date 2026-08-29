package pipeline

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

type Options struct {
	Width        int
	Height       int
	Framerate    int
	VideoBitrate int
	Encoder      string
	CPUThreads   int
	HLSTime      int
	HLSListSize  int
	AudioSource  string
	HLSDir       string
	NodeID       int
	PipeWireFD   int
}

func BuildFFmpegArgs(opts Options) []string {
	gopSize := opts.Framerate * 2
	manifestPath := filepath.Join(opts.HLSDir, "index.m3u8")
	segmentPattern := filepath.Join(opts.HLSDir, "stream_%04d.ts")

	args := []string{
		"-hide_banner", "-loglevel", "info",
		"-fflags", "+genpts+nobuffer+flush_packets",
		"-thread_queue_size", "4096",
		"-f", "rawvideo",
		"-pix_fmt", "yuv420p",
		"-s", fmt.Sprintf("%dx%d", opts.Width, opts.Height),
		"-r", strconv.Itoa(opts.Framerate),
		"-i", "-",
	}

	if opts.AudioSource != "none" && opts.AudioSource != "" {
		args = append(args,
			"-thread_queue_size", "4096",
			"-f", "pulse",
			"-fragment_size", "1024",
			"-i", opts.AudioSource,
			"-c:a", "aac",
			"-b:a", "192k",
			"-ar", "48000",
			"-af", "aresample=async=1:first_pts=0",
		)
	} else {
		args = append(args, "-an")
	}

	threads := opts.CPUThreads
	if threads <= 0 {
		threads = 4
	}

	var videoEncArgs []string
	switch strings.ToLower(opts.Encoder) {
	case "cpu", "x264":
		videoEncArgs = []string{
			"-c:v", "libx264",
			"-preset", "ultrafast",
			"-tune", "zerolatency",
			"-profile:v", "high",
			"-threads", strconv.Itoa(threads),
			"-pix_fmt", "yuv420p",
			"-b:v", fmt.Sprintf("%dk", opts.VideoBitrate),
			"-g", strconv.Itoa(gopSize),
			"-keyint_min", strconv.Itoa(opts.Framerate),
			"-force_key_frames", fmt.Sprintf("expr:gte(t,n_forced*%d)", opts.HLSTime),
			"-sc_threshold", "0",
		}
	case "nvenc":
		videoEncArgs = []string{
			"-c:v", "h264_nvenc",
			"-preset", "p1",
			"-tune", "ull",
			"-b:v", fmt.Sprintf("%dk", opts.VideoBitrate),
			"-g", strconv.Itoa(gopSize),
			"-keyint_min", strconv.Itoa(opts.Framerate),
			"-force_key_frames", fmt.Sprintf("expr:gte(t,n_forced*%d)", opts.HLSTime),
			"-sc_threshold", "0",
		}
	default: // "gpu", "vaapi", "auto"
		videoEncArgs = []string{
			"-vaapi_device", "/dev/dri/renderD128",
			"-vf", "format=nv12,hwupload",
			"-c:v", "h264_vaapi",
			"-b:v", fmt.Sprintf("%dk", opts.VideoBitrate),
			"-g", strconv.Itoa(gopSize),
			"-keyint_min", strconv.Itoa(opts.Framerate),
			"-force_key_frames", fmt.Sprintf("expr:gte(t,n_forced*%d)", opts.HLSTime),
		}
	}

	args = append(args, videoEncArgs...)
	args = append(args,
		"-max_muxing_queue_size", "4096",
		"-f", "hls",
		"-hls_init_time", "1",
		"-hls_time", strconv.Itoa(opts.HLSTime),
		"-hls_list_size", strconv.Itoa(opts.HLSListSize),
		"-hls_flags", "delete_segments+omit_endlist+independent_segments+temp_file",
		"-hls_segment_type", "mpegts",
		"-hls_segment_filename", segmentPattern,
		manifestPath,
	)

	return args
}

func BuildGstPipelineStr(width, height, framerate int) string {
	return fmt.Sprintf(
		"pipewiresrc name=src do-timestamp=true keepalive-time=33 always-copy=true ! "+
			"videoconvert ! videoscale ! videorate drop-only=false skip-to-first=true ! "+
			"video/x-raw,format=I420,width=%d,height=%d,framerate=%d/1 ! "+
			"fdsink name=sink sync=false",
		width, height, framerate,
	)
}

type Runner struct {
	opts       Options
	ffmpegCmd  *exec.Cmd
	gstCmd     *exec.Cmd
	cancelFunc context.CancelFunc
}

func NewRunner(opts Options) *Runner {
	return &Runner{opts: opts}
}

func (r *Runner) Start(ctx context.Context) error {
	_ = os.MkdirAll(r.opts.HLSDir, 0755)

	// Clean previous files
	entries, _ := os.ReadDir(r.opts.HLSDir)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".ts" || filepath.Ext(e.Name()) == ".m3u8" {
			_ = os.Remove(filepath.Join(r.opts.HLSDir, e.Name()))
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	r.cancelFunc = cancel

	// In Go os/exec, ExtraFiles[0] is inherited as FD 3 in the child process
	gstArgs := []string{
		"-q",
		"pipewiresrc",
		"fd=3",
		fmt.Sprintf("path=%d", r.opts.NodeID),
		"do-timestamp=true",
		"keepalive-time=33",
		"always-copy=true",
		"!", "queue", "max-size-buffers=3", "leaky=downstream",
		"!", "videoconvert",
		"!", "videoscale",
		"!", "videorate", "drop-only=false", "skip-to-first=true",
		"!", fmt.Sprintf("video/x-raw,format=I420,width=%d,height=%d,framerate=%d/1", r.opts.Width, r.opts.Height, r.opts.Framerate),
		"!", "fdsink", "fd=1", "sync=true",
	}

	r.gstCmd = exec.CommandContext(ctx, "gst-launch-1.0", gstArgs...)
	r.gstCmd.ExtraFiles = []*os.File{os.NewFile(uintptr(r.opts.PipeWireFD), "pipewire-fd")}
	r.gstCmd.Stderr = os.Stderr

	pipeReader, pipeWriter, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("failed to create pipe: %w", err)
	}

	r.gstCmd.Stdout = pipeWriter

	ffmpegArgs := BuildFFmpegArgs(r.opts)
	r.ffmpegCmd = exec.CommandContext(ctx, "ffmpeg", ffmpegArgs...)
	r.ffmpegCmd.Stdin = pipeReader
	r.ffmpegCmd.Stdout = os.Stdout
	r.ffmpegCmd.Stderr = os.Stderr

	if err := r.gstCmd.Start(); err != nil {
		return fmt.Errorf("failed to start gst-launch-1.0: %w", err)
	}

	if err := r.ffmpegCmd.Start(); err != nil {
		_ = r.gstCmd.Process.Kill()
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	// Close parent ends of the pipe
	_ = pipeWriter.Close()

	return nil
}

func (r *Runner) Wait() error {
	if r.ffmpegCmd != nil {
		return r.ffmpegCmd.Wait()
	}
	return nil
}

func (r *Runner) Stop() {
	if r.cancelFunc != nil {
		r.cancelFunc()
	}
	if r.ffmpegCmd != nil && r.ffmpegCmd.Process != nil {
		_ = r.ffmpegCmd.Process.Signal(syscall.SIGTERM)
	}
	if r.gstCmd != nil && r.gstCmd.Process != nil {
		_ = r.gstCmd.Process.Signal(syscall.SIGTERM)
	}
}
