package audio

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type Filter struct {
	blacklist  []string
	includeMic bool
}

func NewFilter(blacklist []string, includeMic bool) *Filter {
	var clean []string
	for _, b := range blacklist {
		if c := strings.ToLower(strings.TrimSpace(b)); c != "" {
			clean = append(clean, c)
		}
	}
	return &Filter{
		blacklist:  clean,
		includeMic: includeMic,
	}
}

func (f *Filter) IsBlacklisted(appName string) bool {
	lower := strings.ToLower(appName)
	for _, b := range f.blacklist {
		if strings.Contains(lower, b) {
			return true
		}
	}
	return false
}

func IsMicrophone(sourceName string) bool {
	lower := strings.ToLower(sourceName)
	if strings.Contains(lower, ".monitor") {
		return false
	}
	if strings.Contains(lower, "input") || strings.Contains(lower, "mic") || strings.Contains(lower, "source") {
		return true
	}
	return false
}

func GetDesktopMonitorSource() string {
	cmd := exec.Command("pactl", "get-default-sink")
	out, err := cmd.Output()
	if err == nil {
		sink := strings.TrimSpace(string(out))
		if sink != "" && sink != "stream_sink" {
			return sink + ".monitor"
		}
	}

	// Fallback to first non-stream_sink sink
	sinksOut, _ := exec.Command("pactl", "list", "short", "sinks").Output()
	for _, line := range strings.Split(string(sinksOut), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && !strings.Contains(fields[1], "stream_sink") {
			return fields[1] + ".monitor"
		}
	}
	return "stream_sink.monitor"
}

type Router struct {
	filter              *Filter
	enabled             bool
	sinkName            string
	physicalSink        string
	nullSinkModuleID    string
	loopbackModuleID    string
	originalDefaultSink string
	cancelFunc          context.CancelFunc
	mu                  sync.Mutex
}

func NewRouter(filter *Filter, enabled bool) *Router {
	return &Router{
		filter:   filter,
		enabled:  enabled,
		sinkName: "stream_sink",
	}
}

func (r *Router) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.enabled {
		fmt.Println("[AudioRouter] Mirror Mode: Audio routing disabled. Capturing desktop audio directly without modifying sinks or apps.")
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	r.cancelFunc = cancel

	if err := r.setupRouting(); err != nil {
		fmt.Printf("[AudioRouter] Notice: %v\n", err)
	}

	go r.monitorLoop(ctx)
	return nil
}

func (r *Router) setupRouting() error {
	// 1. Ensure stream_sink exists
	checkCmd := exec.Command("pactl", "list", "short", "sinks")
	sinksList, _ := checkCmd.Output()
	if !strings.Contains(string(sinksList), r.sinkName) {
		loadCmd := exec.Command("pactl", "load-module", "module-null-sink",
			fmt.Sprintf("sink_name=%s", r.sinkName),
			"sink_properties=device.description=StreamAudioSink",
		)
		if modOut, err := loadCmd.Output(); err == nil {
			r.nullSinkModuleID = strings.TrimSpace(string(modOut))
		}
	}

	// 2. We deliberately DO NOT create any loopback module to physicalSink!
	// Native PipeWire direct linking mirrors application audio directly to stream_sink
	// without any loopback echo, delay, or doubled audio in headphones.

	// 3. Immediately perform a sync
	r.syncRoutes()
	return nil
}

func (r *Router) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.syncRoutes()
		}
	}
}

func (r *Router) syncRoutes() {
	cmd := exec.Command("pw-link", "-o")
	out, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			port := strings.TrimSpace(line)
			if port == "" {
				continue
			}

			// Filter out monitor, capture, and system non-app ports
			if strings.Contains(port, "monitor_") ||
				strings.Contains(port, "capture_") ||
				strings.HasPrefix(port, "stream_sink:") ||
				strings.HasPrefix(port, "alsa_") ||
				strings.HasPrefix(port, "ee_") ||
				strings.HasPrefix(port, "easyeffects_") ||
				strings.HasPrefix(port, "mitsu_") ||
				strings.HasPrefix(port, "Midi-Bridge") ||
				strings.HasPrefix(port, "bluez_") ||
				strings.HasPrefix(port, "PulseAudio") ||
				strings.HasPrefix(port, "gst-launch") ||
				strings.HasPrefix(port, "xdg-desktop-portal") {
				continue
			}

			nodeName := port
			if idx := strings.Index(port, ":"); idx != -1 {
				nodeName = port[:idx]
			}

			if r.filter.IsBlacklisted(nodeName) {
				// Blacklisted app (e.g. Discord, WEBRTC VoiceEngine) -> Ensure disconnected from stream_sink
				if strings.HasSuffix(port, "_FL") || strings.HasSuffix(port, ":output_0") {
					_ = exec.Command("pw-link", "-d", port, "stream_sink:playback_FL").Run()
				}
				if strings.HasSuffix(port, "_FR") || strings.HasSuffix(port, ":output_1") {
					_ = exec.Command("pw-link", "-d", port, "stream_sink:playback_FR").Run()
				}
			} else {
				// Non-blacklisted app (e.g. CS2, Chrome, Spotify) -> Link directly into stream_sink
				if strings.HasSuffix(port, "_FL") || strings.HasSuffix(port, ":output_0") {
					_ = exec.Command("pw-link", port, "stream_sink:playback_FL").Run()
				} else if strings.HasSuffix(port, "_FR") || strings.HasSuffix(port, ":output_1") {
					_ = exec.Command("pw-link", port, "stream_sink:playback_FR").Run()
				} else if strings.HasSuffix(port, "_MONO") || strings.HasSuffix(port, ":output") {
					_ = exec.Command("pw-link", port, "stream_sink:playback_FL").Run()
					_ = exec.Command("pw-link", port, "stream_sink:playback_FR").Run()
				}
			}
		}
	}
}

func (r *Router) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cancelFunc != nil {
		r.cancelFunc()
	}

	if r.nullSinkModuleID != "" {
		_ = exec.Command("pactl", "unload-module", r.nullSinkModuleID).Run()
		r.nullSinkModuleID = ""
	}
}
