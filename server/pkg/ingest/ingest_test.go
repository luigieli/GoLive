package ingest

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/luigieli/streaming/server/pkg/hub"
)

func TestHTTPIngestAuthorized(t *testing.T) {
	wsHub := hub.NewWSHub()
	go wsHub.Run()

	client := hub.NewWSClient(wsHub, nil)
	wsHub.Register(client)
	time.Sleep(20 * time.Millisecond)

	handler := NewHTTPHandler(wsHub, "my-secret-key")

	payload := []byte("INGEST_PACKET_MPEGTS_PAYLOAD_1234567890")
	req := httptest.NewRequest(http.MethodPost, "/api/publish?key=my-secret-key", bytes.NewReader(payload))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHTTPIngestUnauthorized(t *testing.T) {
	wsHub := hub.NewWSHub()
	go wsHub.Run()

	handler := NewHTTPHandler(wsHub, "my-secret-key")

	req := httptest.NewRequest(http.MethodPost, "/api/publish?key=wrong-key", bytes.NewReader([]byte("test")))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 Unauthorized, got %d", w.Code)
	}
}
