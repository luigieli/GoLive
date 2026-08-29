package config

import (
	"os"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	os.Clearenv()
	cfg := Load()

	if cfg.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Port)
	}
	if cfg.Framerate != 60 {
		t.Errorf("expected default framerate 60, got %d", cfg.Framerate)
	}
	if cfg.VideoBitrate != 6000 {
		t.Errorf("expected default bitrate 6000, got %d", cfg.VideoBitrate)
	}
	if cfg.Encoder != "gpu" {
		t.Errorf("expected default encoder gpu, got %s", cfg.Encoder)
	}
	if !cfg.AudioRouting {
		t.Errorf("expected default AudioRouting true, got false")
	}
	if cfg.IncludeMic != false {
		t.Errorf("expected default includeMic false, got %v", cfg.IncludeMic)
	}
}

func TestCustomEnvConfig(t *testing.T) {
	os.Setenv("PORT", "9090")
	os.Setenv("FRAMERATE", "30")
	os.Setenv("VIDEO_BITRATE", "4000k")
	os.Setenv("ENCODER", "cpu")
	os.Setenv("AUDIO_ROUTING", "false")
	os.Setenv("INCLUDE_MIC", "true")
	os.Setenv("AUDIO_BLACKLIST", "app1,app2")
	defer os.Clearenv()

	cfg := Load()
	if cfg.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Port)
	}
	if cfg.Framerate != 30 {
		t.Errorf("expected framerate 30, got %d", cfg.Framerate)
	}
	if cfg.VideoBitrate != 4000 {
		t.Errorf("expected bitrate 4000, got %d", cfg.VideoBitrate)
	}
	if cfg.Encoder != "cpu" {
		t.Errorf("expected encoder cpu, got %s", cfg.Encoder)
	}
	if cfg.AudioRouting {
		t.Errorf("expected AudioRouting false, got true")
	}
	if cfg.IncludeMic != true {
		t.Errorf("expected includeMic true, got %v", cfg.IncludeMic)
	}
	if len(cfg.AudioBlacklist) != 2 || cfg.AudioBlacklist[0] != "app1" {
		t.Errorf("unexpected blacklist: %v", cfg.AudioBlacklist)
	}
}
