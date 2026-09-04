package config

import (
	"os"
	"testing"
)

func TestGetEnvString(t *testing.T) {
	os.Setenv("TEST_KEY_STR", "hello_world")
	defer os.Unsetenv("TEST_KEY_STR")

	if val := GetEnvString("TEST_KEY_STR", "fallback"); val != "hello_world" {
		t.Errorf("expected hello_world, got %s", val)
	}
	if val := GetEnvString("NON_EXISTENT_KEY", "fallback"); val != "fallback" {
		t.Errorf("expected fallback, got %s", val)
	}
}

func TestGetEnvInt(t *testing.T) {
	os.Setenv("TEST_KEY_INT", "1234")
	defer os.Unsetenv("TEST_KEY_INT")

	if val := GetEnvInt("TEST_KEY_INT", 99); val != 1234 {
		t.Errorf("expected 1234, got %d", val)
	}
	if val := GetEnvInt("NON_EXISTENT_INT", 99); val != 99 {
		t.Errorf("expected 99, got %d", val)
	}
	os.Setenv("INVALID_INT", "abc")
	defer os.Unsetenv("INVALID_INT")
	if val := GetEnvInt("INVALID_INT", 99); val != 99 {
		t.Errorf("expected fallback for invalid int, got %d", val)
	}
}

func TestGetEnvBool(t *testing.T) {
	os.Setenv("TEST_KEY_BOOL", "true")
	defer os.Unsetenv("TEST_KEY_BOOL")

	if val := GetEnvBool("TEST_KEY_BOOL", false); !val {
		t.Errorf("expected true, got false")
	}
	if val := GetEnvBool("NON_EXISTENT_BOOL", true); !val {
		t.Errorf("expected fallback true, got false")
	}
}

func TestParseBitrate(t *testing.T) {
	cases := []struct {
		input    string
		expected int
	}{
		{"6000", 6000},
		{"6000k", 6000},
		{"10000kbps", 10000},
		{"invalid", 6000},
		{"-500", 6000},
	}
	for _, c := range cases {
		if res := ParseBitrate(c.input); res != c.expected {
			t.Errorf("for input %q expected %d, got %d", c.input, c.expected, res)
		}
	}
}

func TestParseList(t *testing.T) {
	raw := "item1, item2,  item3 , "
	res := ParseList(raw)
	if len(res) != 3 {
		t.Fatalf("expected 3 items, got %d: %v", len(res), res)
	}
	if res[0] != "item1" || res[1] != "item2" || res[2] != "item3" {
		t.Errorf("unexpected list elements: %v", res)
	}
}
