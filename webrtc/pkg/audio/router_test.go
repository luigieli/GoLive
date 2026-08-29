package audio

import (
	"testing"
)

func TestIsBlacklisted(t *testing.T) {
	blacklist := []string{"discord", "slack", "zoom", "teams"}
	filter := NewFilter(blacklist, false)

	tests := []struct {
		appName  string
		expected bool
	}{
		{"Discord", true},
		{"discord-canary", true},
		{"VESKTOP", false},
		{"Slack", true},
		{"Zoom Meeting", true},
		{"Spotify", false},
		{"Firefox", false},
		{"Chrome", false},
		{"VLC media player", false},
	}

	for _, tt := range tests {
		result := filter.IsBlacklisted(tt.appName)
		if result != tt.expected {
			t.Errorf("IsBlacklisted(%q) = %v; want %v", tt.appName, result, tt.expected)
		}
	}
}

func TestFilterWithVesktop(t *testing.T) {
	blacklist := []string{"discord", "vesktop", "webcord"}
	filter := NewFilter(blacklist, false)

	if !filter.IsBlacklisted("Vesktop") {
		t.Errorf("expected Vesktop to be blacklisted")
	}
	if !filter.IsBlacklisted("webcord_bin") {
		t.Errorf("expected webcord_bin to be blacklisted")
	}
	if filter.IsBlacklisted("OBS Studio") {
		t.Errorf("expected OBS Studio NOT to be blacklisted")
	}
}

func TestIsMicrophone(t *testing.T) {
	tests := []struct {
		sourceName string
		isMic      bool
	}{
		{"alsa_input.usb-HP__Inc_HyperX_SoloCast-00.iec958-stereo", true},
		{"denoised_mic", true},
		{"easyeffects_source", true},
		{"alsa_input.pci-0000_15_00.4.analog-stereo", true},
		{"alsa_output.pci-0000_15_00.4.analog-stereo.monitor", false},
		{"stream_sink.monitor", false},
		{"easyeffects_sink.monitor", false},
	}

	for _, tt := range tests {
		result := IsMicrophone(tt.sourceName)
		if result != tt.isMic {
			t.Errorf("IsMicrophone(%q) = %v; want %v", tt.sourceName, result, tt.isMic)
		}
	}
}

func TestRouterMirrorMode(t *testing.T) {
	filter := NewFilter([]string{"discord"}, false)
	router := NewRouter(filter, false)
	if router.enabled {
		t.Errorf("expected router to be disabled in mirror mode")
	}
}
