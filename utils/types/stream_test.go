package types

import (
	"testing"
)

func TestStreamInfoDefaults(t *testing.T) {
	info := StreamInfo{
		NodeID:     10,
		PipeWireFD: 3,
		Width:      1920,
		Height:     1080,
	}

	if info.Width != 1920 || info.Height != 1080 {
		t.Errorf("unexpected resolution %dx%d", info.Width, info.Height)
	}
	if info.NodeID != 10 || info.PipeWireFD != 3 {
		t.Errorf("unexpected IDs NodeID=%d, PipeWireFD=%d", info.NodeID, info.PipeWireFD)
	}
}
