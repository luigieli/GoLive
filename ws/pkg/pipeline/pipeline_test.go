package pipeline

import (
	"strings"
	"testing"
)

func TestBuildGstArgsCPU(t *testing.T) {
	opts := Options{
		SourceWidth:  3840,
		SourceHeight: 2160,
		TargetWidth:  1920,
		TargetHeight: 1080,
		Framerate:    60,
		VideoBitrate: 6000,
		Encoder:      "cpu",
		NodeID:       42,
		PipeWireFD:   7,
		AudioSource:  "stream_sink.monitor",
	}

	runner := NewRunner(opts, nil)
	args := runner.buildGstArgs()
	cmdStr := strings.Join(args, " ")

	if !strings.Contains(cmdStr, "pipewiresrc") {
		t.Errorf("expected pipewiresrc in gst args, got: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "path=42") {
		t.Errorf("expected path=42 in gst args, got: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "x264enc") {
		t.Errorf("expected x264enc in gst args, got: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "format=I420") {
		t.Errorf("expected format=I420 in cpu gst args, got: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "constrained-baseline") {
		t.Errorf("expected constrained-baseline in gst args, got: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "avenc_aac") {
		t.Errorf("expected avenc_aac in gst args, got: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "mpegtsmux") {
		t.Errorf("expected mpegtsmux in gst args, got: %s", cmdStr)
	}
}

func TestBuildGstArgsGPU(t *testing.T) {
	opts := Options{
		SourceWidth:  3840,
		SourceHeight: 2160,
		TargetWidth:  1920,
		TargetHeight: 1080,
		Framerate:    60,
		VideoBitrate: 6000,
		Encoder:      "gpu",
		NodeID:       42,
		PipeWireFD:   7,
		AudioSource:  "stream_sink.monitor",
	}

	runner := NewRunner(opts, nil)
	args := runner.buildGstArgs()
	cmdStr := strings.Join(args, " ")

	if !strings.Contains(cmdStr, "pipewiresrc") {
		t.Errorf("expected pipewiresrc in gst args, got: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "vaapih264enc") {
		t.Errorf("expected vaapih264enc in gst args, got: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "format=NV12") {
		t.Errorf("expected format=NV12 in gpu gst args, got: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "rate-control=cbr") {
		t.Errorf("expected rate-control=cbr in gpu gst args, got: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "mpegtsmux") {
		t.Errorf("expected mpegtsmux in gst args, got: %s", cmdStr)
	}
}

func TestBuildGstArgsNVENC(t *testing.T) {
	opts := Options{
		SourceWidth:  1920,
		SourceHeight: 1080,
		Framerate:    60,
		VideoBitrate: 8000,
		Encoder:      "nvenc",
		NodeID:       42,
		PipeWireFD:   7,
		AudioSource:  "stream_sink.monitor",
	}

	runner := NewRunner(opts, nil)
	args := runner.buildGstArgs()
	cmdStr := strings.Join(args, " ")

	if !strings.Contains(cmdStr, "nvh264enc") {
		t.Errorf("expected nvh264enc in gst args, got: %s", cmdStr)
	}
}
