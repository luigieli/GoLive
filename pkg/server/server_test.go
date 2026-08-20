package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServerRoot(t *testing.T) {
	tempDir := t.TempDir()
	srv := NewServer(tempDir)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("expected text/html content-type, got %s", w.Header().Get("Content-Type"))
	}
}

func TestServerHLSNotFound(t *testing.T) {
	tempDir := t.TempDir()
	srv := NewServer(tempDir)

	req := httptest.NewRequest("GET", "/hls/index.m3u8", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for missing manifest, got %d", w.Code)
	}
}

func TestServerHLSSuccess(t *testing.T) {
	tempDir := t.TempDir()
	manifestFile := filepath.Join(tempDir, "index.m3u8")
	_ = os.WriteFile(manifestFile, []byte("#EXTM3U\n#EXT-X-VERSION:3"), 0644)

	segmentFile := filepath.Join(tempDir, "stream_0000.ts")
	_ = os.WriteFile(segmentFile, []byte("dummy-ts-data"), 0644)

	srv := NewServer(tempDir)

	// Test Manifest
	reqM := httptest.NewRequest("GET", "/hls/index.m3u8", nil)
	wM := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wM, reqM)

	if wM.Code != http.StatusOK {
		t.Errorf("expected status 200 for manifest, got %d", wM.Code)
	}
	if wM.Header().Get("Content-Type") != "application/vnd.apple.mpegurl" {
		t.Errorf("expected m3u8 content-type, got %s", wM.Header().Get("Content-Type"))
	}
	if wM.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected CORS header, got %s", wM.Header().Get("Access-Control-Allow-Origin"))
	}
	if !strings.Contains(wM.Header().Get("Cache-Control"), "no-cache") {
		t.Errorf("expected no-cache header for m3u8, got %s", wM.Header().Get("Cache-Control"))
	}

	// Test Segment
	reqS := httptest.NewRequest("GET", "/hls/stream_0000.ts", nil)
	wS := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wS, reqS)

	if wS.Code != http.StatusOK {
		t.Errorf("expected status 200 for segment, got %d", wS.Code)
	}
	if wS.Header().Get("Content-Type") != "video/mp2t" {
		t.Errorf("expected video/mp2t content-type, got %s", wS.Header().Get("Content-Type"))
	}
}
