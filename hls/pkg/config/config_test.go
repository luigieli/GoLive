package config

import (
	"os"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	os.Clearenv()
	cfg := Load()

	if cfg.Port != 8080 {
		t.Errorf("expected default Port to be 8080, got %d", cfg.Port)
	}
	if cfg.Framerate != 30 {
		t.Errorf("expected default Framerate to be 30, got %d", cfg.Framerate)
	}
	if cfg.VideoBitrate != 6000 {
		t.Errorf("expected default VideoBitrate to be 6000, got %d", cfg.VideoBitrate)
	}
	if cfg.Encoder != "gpu" {
		t.Errorf("expected default Encoder to be gpu, got %s", cfg.Encoder)
	}
	if !cfg.AudioRouting {
		t.Errorf("expected default AudioRouting to be true, got false")
	}
	if cfg.HLSTime != 2 {
		t.Errorf("expected default HLSTime to be 2, got %d", cfg.HLSTime)
	}
	if cfg.HLSListSize != 10 {
		t.Errorf("expected default HLSListSize to be 10, got %d", cfg.HLSListSize)
	}
	if cfg.IncludeMic != false {
		t.Errorf("expected default IncludeMic to be false, got %v", cfg.IncludeMic)
	}
	if len(cfg.AudioBlacklist) == 0 {
		t.Errorf("expected non-empty default AudioBlacklist")
	}
}

func TestCustomEnvConfig(t *testing.T) {
	os.Setenv("PORT", "9090")
	os.Setenv("FRAMERATE", "60")
	os.Setenv("VIDEO_BITRATE", "8000k")
	os.Setenv("ENCODER", "cpu")
	os.Setenv("AUDIO_ROUTING", "false")
	os.Setenv("HLS_TIME", "1")
	os.Setenv("HLS_LIST_SIZE", "10")
	os.Setenv("INCLUDE_MIC", "true")
	os.Setenv("AUDIO_BLACKLIST", "app1,App2,app3")
	os.Setenv("HLS_DIR", "/custom/hls")

	cfg := Load()

	if cfg.Port != 9090 {
		t.Errorf("expected Port 9090, got %d", cfg.Port)
	}
	if cfg.Framerate != 60 {
		t.Errorf("expected Framerate 60, got %d", cfg.Framerate)
	}
	if cfg.VideoBitrate != 8000 {
		t.Errorf("expected VideoBitrate 8000, got %d", cfg.VideoBitrate)
	}
	if cfg.Encoder != "cpu" {
		t.Errorf("expected Encoder cpu, got %s", cfg.Encoder)
	}
	if cfg.AudioRouting {
		t.Errorf("expected AudioRouting false, got true")
	}
	if cfg.HLSTime != 1 {
		t.Errorf("expected HLSTime 1, got %d", cfg.HLSTime)
	}
	if cfg.HLSListSize != 10 {
		t.Errorf("expected HLSListSize 10, got %d", cfg.HLSListSize)
	}
	if cfg.IncludeMic != true {
		t.Errorf("expected IncludeMic true, got %v", cfg.IncludeMic)
	}
	if len(cfg.AudioBlacklist) != 3 || cfg.AudioBlacklist[0] != "app1" || cfg.AudioBlacklist[1] != "app2" {
		t.Errorf("expected parsed AudioBlacklist, got %v", cfg.AudioBlacklist)
	}
	if cfg.HLSDir != "/custom/hls" {
		t.Errorf("expected HLSDir /custom/hls, got %s", cfg.HLSDir)
	}
}
