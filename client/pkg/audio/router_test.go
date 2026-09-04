package audio

import (
	"testing"
)

func TestIsBlacklisted(t *testing.T) {
	filter := NewFilter([]string{"discord", "slack", "zoom"}, false)

	if !filter.IsBlacklisted("Discord") {
		t.Errorf("expected Discord to be blacklisted")
	}
	if !filter.IsBlacklisted("slack-app") {
		t.Errorf("expected slack-app to be blacklisted")
	}
	if filter.IsBlacklisted("firefox") {
		t.Errorf("expected firefox to not be blacklisted")
	}
}

func TestFilterWithVesktop(t *testing.T) {
	filter := NewFilter([]string{"discord", "vesktop"}, false)

	if !filter.IsBlacklisted("vesktop.bin") {
		t.Errorf("expected vesktop to be blacklisted")
	}
}

func TestIsMicrophone(t *testing.T) {
	if !IsMicrophone("alsa_input.pci-0000_00_1f.3.analog-stereo") {
		t.Errorf("expected alsa_input to be recognized as microphone")
	}
	if IsMicrophone("alsa_output.pci-0000_00_1f.3.analog-stereo.monitor") {
		t.Errorf("expected .monitor to NOT be recognized as microphone")
	}
}

func TestRouterMirrorMode(t *testing.T) {
	filter := NewFilter([]string{"discord"}, false)
	router := NewRouter(filter, false)

	if router.enabled {
		t.Errorf("expected router to be disabled in mirror mode")
	}
}
