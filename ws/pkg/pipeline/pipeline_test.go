package pipeline

import (
	"strings"
	"testing"
)

func TestBuildGstArgs(t *testing.T) {
	opts := Options{
		SourceWidth:  3840,
		SourceHeight: 2160,
		TargetWidth:  1920,
		TargetHeight: 1080,
		Framerate:    60,
		VideoBitrate: 6000,
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
