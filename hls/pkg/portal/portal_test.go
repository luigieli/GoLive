package portal

import (
	"strings"
	"testing"
)

func TestGenerateToken(t *testing.T) {
	tok1 := generateToken()
	tok2 := generateToken()

	if !strings.HasPrefix(tok1, "u_") {
		t.Errorf("expected token to start with 'u_', got %s", tok1)
	}
	if tok1 == tok2 {
		t.Errorf("expected unique tokens, got identical: %s", tok1)
	}
}

func TestParseStreamProperties(t *testing.T) {
	rawStreams := []interface{}{
		[]interface{}{
			uint32(42),
			map[string]interface{}{
				"size": []interface{}{int32(1920), int32(1080)},
			},
		},
	}

	stream, err := ParseStreams(rawStreams)
	if err != nil {
		t.Fatalf("unexpected error parsing streams: %v", err)
	}

	if stream.NodeID != 42 {
		t.Errorf("expected NodeID 42, got %d", stream.NodeID)
	}
	if stream.Width != 1920 || stream.Height != 1080 {
		t.Errorf("expected resolution 1920x1080, got %dx%d", stream.Width, stream.Height)
	}
}

func TestParseStreamPropertiesDefaultSize(t *testing.T) {
	rawStreams := []interface{}{
		[]interface{}{
			uint32(100),
			map[string]interface{}{},
		},
	}

	stream, err := ParseStreams(rawStreams)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stream.NodeID != 100 {
		t.Errorf("expected NodeID 100, got %d", stream.NodeID)
	}
	if stream.Width != 3840 || stream.Height != 2160 {
		t.Errorf("expected fallback resolution 3840x2160, got %dx%d", stream.Width, stream.Height)
	}
}
