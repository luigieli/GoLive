package config

import (
	"os"
	"strconv"
	"strings"
	"unicode"
)

func GetEnvString(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val)
	}
	return fallback
}

func GetEnvInt(key string, fallback int) int {
	if val, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(strings.TrimSpace(val)); err == nil && i > 0 {
			return i
		}
	}
	return fallback
}

func GetEnvBool(key string, fallback bool) bool {
	if val, ok := os.LookupEnv(key); ok {
		lower := strings.ToLower(strings.TrimSpace(val))
		return lower == "true" || lower == "1" || lower == "yes"
	}
	return fallback
}

func ParseBitrate(val string) int {
	val = strings.TrimSpace(val)
	if strings.HasPrefix(val, "-") {
		return 6000
	}
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

func ParseList(raw string) []string {
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
