package portal

import (
	"testing"
)

func TestParseStreamProperties(t *testing.T) {
	props := map[string]interface{}{
		"size": []interface{}{uint32(2560), uint32(1440)},
	}

	w, h := parseStreamProperties(props)
	if w != 2560 || h != 1440 {
		t.Errorf("expected 2560x1440, got %dx%d", w, h)
	}
}

func TestParseStreamPropertiesDefaultSize(t *testing.T) {
	props := map[string]interface{}{}
	w, h := parseStreamProperties(props)
	if w != 1920 || h != 1080 {
		t.Errorf("expected default 1920x1080, got %dx%d", w, h)
	}
}
