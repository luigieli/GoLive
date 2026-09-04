package pipeline

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSenderStreamData(t *testing.T) {
	receivedData := make([]byte, 0)
	done := make(chan bool)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		if key != "secret-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		buf := make([]byte, 1024)
		for {
			n, err := r.Body.Read(buf)
			if n > 0 {
				receivedData = append(receivedData, buf[:n]...)
			}
			if err != nil {
				break
			}
		}
		w.WriteHeader(http.StatusOK)
		close(done)
	}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	payload := []byte("STREAM_TEST_PACKET_BYTES_1234567890")
	streamReader := bytes.NewReader(payload)

	sender := NewSender(ts.URL+"/api/publish", "secret-token")
	err := sender.Send(ctx, streamReader)
	if err != nil {
		t.Fatalf("Sender.Send failed: %v", err)
	}

	select {
	case <-done:
		if !bytes.Equal(receivedData, payload) {
			t.Errorf("expected %s, got %s", payload, receivedData)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("timed out waiting for server to receive data")
	}
}

func TestSenderUnauthorized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	sender := NewSender(ts.URL+"/api/publish", "wrong-token")
	err := sender.Send(ctx, io.NopCloser(bytes.NewReader([]byte("test"))))
	if err == nil {
		t.Errorf("expected error on unauthorized response, got nil")
	}
}
