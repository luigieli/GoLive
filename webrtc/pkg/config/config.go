package config

import (
	"os"
	"strconv"
	"strings"
	"unicode"
)

type Config struct {
	Port           int
	Framerate      int
	VideoBitrate   int
	IncludeMic     bool
	AudioBlacklist []string
	AudioSource    string
	ICEServers     []string
}

func Load() *Config {
	port := getEnvInt("PORT", 8080)
	framerate := getEnvInt("FRAMERATE", 60)
	bitrate := parseBitrate(getEnvString("VIDEO_BITRATE", "6000"))
	includeMic := getEnvBool("INCLUDE_MIC", false)

	defaultBlacklist := "discord,Discord,vesktop,webcord,slack,zoom,teams"
	rawBlacklist := getEnvString("AUDIO_BLACKLIST", defaultBlacklist)
	blacklist := parseList(rawBlacklist)

	audioSource := getEnvString("AUDIO_SOURCE", "stream_sink.monitor")

	defaultSTUN := "stun:stun.l.google.com:19302,stun:stun1.l.google.com:19302"
	rawSTUN := getEnvString("STUN_SERVERS", defaultSTUN)
	iceServers := parseList(rawSTUN)

	return &Config{
		Port:           port,
		Framerate:      framerate,
		VideoBitrate:   bitrate,
		IncludeMic:     includeMic,
		AudioBlacklist: blacklist,
		AudioSource:    audioSource,
		ICEServers:     iceServers,
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

func parseList(raw string) []string {
	parts := strings.Split(raw, ",")
	var result []string
	for _, p := range parts {
		clean := strings.TrimSpace(p)
		if clean != "" {
			result = append(result, clean)
		}
	}
	return result
}

func getEnvString(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val)
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(strings.TrimSpace(val)); err == nil && i > 0 {
			return i
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if val, ok := os.LookupEnv(key); ok {
		lower := strings.ToLower(strings.TrimSpace(val))
		return lower == "true" || lower == "1" || lower == "yes"
	}
	return fallback
}
