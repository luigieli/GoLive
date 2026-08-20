package pipeline

import (
	"strings"
	"testing"
)

func TestBuildGstArgs(t *testing.T) {
	opts := Options{
		Width:        1920,
		Height:       1080,
		Framerate:    60,
		VideoBitrate: 6000,
		NodeID:       42,
		PipeWireFD:   7,
		AudioSource:  "stream_sink.monitor",
		VideoUDPPort: 5004,
		AudioUDPPort: 5006,
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
	if !strings.Contains(cmdStr, "tune=zerolatency") {
		t.Errorf("expected tune=zerolatency in gst args, got: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "rtph264pay") {
		t.Errorf("expected rtph264pay in gst args, got: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "port=5004") {
		t.Errorf("expected port=5004 in gst args, got: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "pulsesrc") {
		t.Errorf("expected pulsesrc in gst args, got: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "opusenc") {
		t.Errorf("expected opusenc in gst args, got: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "port=5006") {
		t.Errorf("expected port=5006 in gst args, got: %s", cmdStr)
	}
}
