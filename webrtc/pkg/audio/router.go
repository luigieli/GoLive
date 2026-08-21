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
		if sink != "" {
			return sink + ".monitor"
		}
	}
	return "stream_sink.monitor"
}

type Router struct {
	filter              *Filter
	sinkName            string
	physicalSink        string
	nullSinkModuleID    string
	loopbackModuleID    string
	originalDefaultSink string
	cancelFunc          context.CancelFunc
	mu                  sync.Mutex
}

func NewRouter(filter *Filter) *Router {
	return &Router{
		filter:   filter,
		sinkName: "stream_sink",
	}
}

func (r *Router) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	ctx, cancel := context.WithCancel(ctx)
	r.cancelFunc = cancel

	if err := r.setupRouting(); err != nil {
		fmt.Printf("[AudioRouter] Notice: %v\n", err)
	}

	go r.monitorLoop(ctx)
	return nil
}

func (r *Router) setupRouting() error {
	// Find current physical default sink
	cmd := exec.Command("pactl", "get-default-sink")
	out, _ := cmd.Output()
	current := strings.TrimSpace(string(out))
	if current != "" && current != r.sinkName {
		r.physicalSink = current
		r.originalDefaultSink = current
	} else if r.physicalSink == "" {
		// Find first non-null sink
		sinksOut, _ := exec.Command("pactl", "list", "short", "sinks").Output()
		for _, line := range strings.Split(string(sinksOut), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 2 && !strings.Contains(fields[1], r.sinkName) {
				r.physicalSink = fields[1]
				r.originalDefaultSink = fields[1]
				break
			}
		}
	}

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

	// 2. Set up loopback from stream_sink.monitor to physicalSink so user hears desktop audio
	if r.physicalSink != "" {
		modsOut, _ := exec.Command("pactl", "list", "short", "modules").Output()
		if !strings.Contains(string(modsOut), fmt.Sprintf("source=%s.monitor", r.sinkName)) {
			loopCmd := exec.Command("pactl", "load-module", "module-loopback",
				fmt.Sprintf("source=%s.monitor", r.sinkName),
				fmt.Sprintf("sink=%s", r.physicalSink),
				"latency_msec=20",
			)
			if loopOut, err := loopCmd.Output(); err == nil {
				r.loopbackModuleID = strings.TrimSpace(string(loopOut))
			}
		}

		// 3. Set stream_sink as default sink so all normal apps play into stream_sink
		_ = exec.Command("pactl", "set-default-sink", r.sinkName).Run()
	}

	// 4. Immediately perform a sync
	r.syncRoutes()
	return nil
}

func (r *Router) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
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
	if r.physicalSink == "" {
		return
	}

	cmd := exec.Command("pactl", "list", "sink-inputs")
	out, err := cmd.Output()
	if err != nil {
		return
	}

	sections := strings.Split(string(out), "Sink Input #")
	for _, sec := range sections {
		if strings.TrimSpace(sec) == "" {
			continue
		}
		lines := strings.Split(sec, "\n")
		var index string

		if len(lines) > 0 {
			index = strings.TrimSpace(lines[0])
		}

		if index == "" {
			continue
		}

		if isLoopbackSinkInput(lines, r.loopbackModuleID) {
			_ = exec.Command("pactl", "move-sink-input", index, r.physicalSink).Run()
			continue
		}

		var appName string
		for _, line := range lines {
			if strings.Contains(line, "application.name =") || strings.Contains(line, "application.process.binary =") {
				parts := strings.Split(line, "=")
				if len(parts) > 1 {
					appName = strings.Trim(strings.TrimSpace(parts[1]), "\"")
				}
			}
		}

		if appName != "" {
			if r.filter.IsBlacklisted(appName) {
				// Blacklisted app (e.g. Discord, Slack) -> Route directly to physical speakers/headphones
				_ = exec.Command("pactl", "move-sink-input", index, r.physicalSink).Run()
			} else {
				// Non-blacklisted app -> Ensure it plays into stream_sink if not already
				_ = exec.Command("pactl", "move-sink-input", index, r.sinkName).Run()
			}
		}
	}
}

func isLoopbackSinkInput(lines []string, loopbackModuleID string) bool {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if loopbackModuleID != "" && (strings.Contains(trimmed, "Owner Module: "+loopbackModuleID) ||
			strings.Contains(trimmed, fmt.Sprintf("pulse.module.id = \"%s\"", loopbackModuleID)) ||
			strings.Contains(trimmed, fmt.Sprintf("pulse.module.id = %s", loopbackModuleID))) {
			return true
		}
		if strings.Contains(trimmed, "node.name =") && strings.Contains(strings.ToLower(trimmed), "loopback") {
			return true
		}
		if strings.Contains(trimmed, "media.name =") && strings.Contains(strings.ToLower(trimmed), "loopback") {
			return true
		}
		if strings.Contains(trimmed, "device.description =") && strings.Contains(strings.ToLower(trimmed), "loopback") {
			return true
		}
		if strings.Contains(trimmed, "module-stream-restore.id =") && strings.Contains(strings.ToLower(trimmed), "loopback") {
			return true
		}
	}
	return false
}

func (r *Router) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cancelFunc != nil {
		r.cancelFunc()
	}

	// Restore original default sink
	if r.originalDefaultSink != "" {
		_ = exec.Command("pactl", "set-default-sink", r.originalDefaultSink).Run()

		// Move all sink inputs back to physical sink
		cmd := exec.Command("pactl", "list", "sink-inputs")
		if out, err := cmd.Output(); err == nil {
			sections := strings.Split(string(out), "Sink Input #")
			for _, sec := range sections {
				lines := strings.Split(sec, "\n")
				if len(lines) > 0 && strings.TrimSpace(lines[0]) != "" {
					index := strings.TrimSpace(lines[0])
					_ = exec.Command("pactl", "move-sink-input", index, r.originalDefaultSink).Run()
				}
			}
		}
	}

	if r.loopbackModuleID != "" {
		_ = exec.Command("pactl", "unload-module", r.loopbackModuleID).Run()
		r.loopbackModuleID = ""
	}
	if r.nullSinkModuleID != "" {
		_ = exec.Command("pactl", "unload-module", r.nullSinkModuleID).Run()
		r.nullSinkModuleID = ""
	}
}
