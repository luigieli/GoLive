package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

type Config struct {
	Port           int
	Framerate      int
	VideoBitrate   int
	HLSTime        int
	HLSListSize    int
	IncludeMic     bool
	AudioBlacklist []string
	HLSDir         string
	AudioSource    string
}

func Load() *Config {
	port := getEnvInt("PORT", 8080)
	framerate := getEnvInt("FRAMERATE", 30)
	bitrate := parseBitrate(getEnvString("VIDEO_BITRATE", "6000"))
	hlsTime := getEnvInt("HLS_TIME", 2)
	hlsListSize := getEnvInt("HLS_LIST_SIZE", 10)
	includeMic := getEnvBool("INCLUDE_MIC", false)

	defaultBlacklist := "discord,Discord,vesktop,webcord,slack,zoom,teams"
	rawBlacklist := getEnvString("AUDIO_BLACKLIST", defaultBlacklist)
	blacklist := parseBlacklist(rawBlacklist)

	defaultHLSDir := filepath.Join(getWorkingDir(), "hls")
	hlsDir := getEnvString("HLS_DIR", defaultHLSDir)
	audioSource := getEnvString("AUDIO_SOURCE", "stream_sink.monitor")

	return &Config{
		Port:           port,
		Framerate:      framerate,
		VideoBitrate:   bitrate,
		HLSTime:        hlsTime,
		HLSListSize:    hlsListSize,
		IncludeMic:     includeMic,
		AudioBlacklist: blacklist,
		HLSDir:         hlsDir,
		AudioSource:    audioSource,
	}
}

func parseBitrate(val string) int {
	var sb strings.Builder
	for _, ch := range val {
		if unicode.IsDigit(ch) {
			sb.WriteRune(ch)
		}
	}
	res, err := strconv.Atoi(sb.String())
	if err != nil || res <= 0 {
		return 6000
	}
	return res
}

func parseBlacklist(val string) []string {
	parts := strings.Split(val, ",")
	seen := make(map[string]bool)
	var res []string
	for _, p := range parts {
		clean := strings.ToLower(strings.TrimSpace(p))
		if clean != "" && !seen[clean] {
			seen[clean] = true
			res = append(res, clean)
		}
	}
	return res
}

func getEnvString(key, def string) string {
	if val := os.Getenv(key); strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val)
	}
	return def
}

func getEnvInt(key string, def int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(strings.TrimSpace(val)); err == nil && i > 0 {
			return i
		}
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	if val := os.Getenv(key); val != "" {
		lower := strings.ToLower(strings.TrimSpace(val))
		return lower == "1" || lower == "true" || lower == "yes"
	}
	return def
}

func getWorkingDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return "/tmp/hls"
	}
	return wd
}
