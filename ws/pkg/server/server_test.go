package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestServerHealth(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	s := NewServer("", hub)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	s.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestServerRoot(t *testing.T) {
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "index.html")
	_ = os.WriteFile(indexPath, []byte("<h1>WS Stream</h1>"), 0644)

	hub := NewHub()
	go hub.Run()
	s := NewServer(tmpDir, hub)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	s.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if w.Body.String() != "<h1>WS Stream</h1>" {
		t.Errorf("unexpected body: %s", w.Body.String())
	}
}
