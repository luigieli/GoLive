package crypto

import (
	"strings"
	"testing"
)

func TestGenerateToken(t *testing.T) {
	t1 := GenerateToken()
	t2 := GenerateToken()

	if len(t1) == 0 {
		t.Errorf("expected non-empty token")
	}
	if !strings.HasPrefix(t1, "u_") {
		t.Errorf("expected token to start with 'u_', got %s", t1)
	}
	if t1 == t2 {
		t.Errorf("expected unique tokens, got identical: %s", t1)
	}
}

func TestValidateKey(t *testing.T) {
	if !ValidateKey("my-secret", "my-secret") {
		t.Errorf("expected matching keys to validate")
	}
	if ValidateKey("my-secret", "wrong-secret") {
		t.Errorf("expected mismatched keys to fail validation")
	}
	// Empty expected key means open/public
	if !ValidateKey("", "any-key") {
		t.Errorf("expected empty required key to allow any key")
	}
}
