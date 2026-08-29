package config

import (
	"os"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	os.Clearenv()
	cfg := Load()

	if cfg.Port != 8080 {
		t.Errorf("expected Port=8080, got %d", cfg.Port)
	}
	if cfg.Framerate != 60 {
		t.Errorf("expected Framerate=60, got %d", cfg.Framerate)
	}
	if cfg.VideoBitrate != 6000 {
		t.Errorf("expected VideoBitrate=6000, got %d", cfg.VideoBitrate)
	}
	if cfg.Encoder != "gpu" {
		t.Errorf("expected Encoder=gpu, got %s", cfg.Encoder)
	}
	if !cfg.AudioRouting {
		t.Errorf("expected AudioRouting=true, got false")
	}
	if cfg.IncludeMic {
		t.Errorf("expected IncludeMic=false, got true")
	}
	if len(cfg.AudioBlacklist) == 0 {
		t.Errorf("expected AudioBlacklist to have defaults")
	}
	if len(cfg.ICEServers) == 0 {
		t.Errorf("expected ICEServers to have defaults")
	}
}

func TestCustomEnvConfig(t *testing.T) {
	os.Clearenv()
	os.Setenv("PORT", "9090")
	os.Setenv("FRAMERATE", "30")
	os.Setenv("VIDEO_BITRATE", "10000k")
	os.Setenv("ENCODER", "cpu")
	os.Setenv("AUDIO_ROUTING", "false")
	os.Setenv("INCLUDE_MIC", "true")
	os.Setenv("AUDIO_BLACKLIST", "app1, app2")
	os.Setenv("STUN_SERVERS", "stun:stun.example.com:3478")

	cfg := Load()

	if cfg.Port != 9090 {
		t.Errorf("expected Port=9090, got %d", cfg.Port)
	}
	if cfg.Framerate != 30 {
		t.Errorf("expected Framerate=30, got %d", cfg.Framerate)
	}
	if cfg.VideoBitrate != 10000 {
		t.Errorf("expected VideoBitrate=10000, got %d", cfg.VideoBitrate)
	}
	if cfg.Encoder != "cpu" {
		t.Errorf("expected Encoder=cpu, got %s", cfg.Encoder)
	}
	if cfg.AudioRouting {
		t.Errorf("expected AudioRouting=false, got true")
	}
	if !cfg.IncludeMic {
		t.Errorf("expected IncludeMic=true, got false")
	}
	if len(cfg.AudioBlacklist) != 2 || cfg.AudioBlacklist[0] != "app1" {
		t.Errorf("expected AudioBlacklist=[app1, app2], got %v", cfg.AudioBlacklist)
	}
	if len(cfg.ICEServers) != 1 || cfg.ICEServers[0] != "stun:stun.example.com:3478" {
		t.Errorf("expected ICEServers=[stun:stun.example.com:3478], got %v", cfg.ICEServers)
	}
}
