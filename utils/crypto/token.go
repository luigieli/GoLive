package crypto

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
)

func GenerateToken() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return "u_" + hex.EncodeToString(b)
}

func ValidateKey(expectedKey, providedKey string) bool {
	if expectedKey == "" {
		return true // Open access if no key configured
	}
	return subtle.ConstantTimeCompare([]byte(expectedKey), []byte(providedKey)) == 1
}
