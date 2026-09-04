package hub

import (
	"testing"
	"time"
)

func TestWSHubBroadcast(t *testing.T) {
	h := NewWSHub()
	go h.Run()

	client := &WSClient{
		hub:  h,
		send: make(chan []byte, 10),
	}

	h.Register(client)

	// Wait for registration
	time.Sleep(20 * time.Millisecond)

	if h.ClientCount() != 1 {
		t.Fatalf("expected 1 client, got %d", h.ClientCount())
	}

	payload := []byte("HELLO_WEBSOCKET_TEST")
	h.Broadcast(payload)

	select {
	case msg := <-client.send:
		if string(msg) != string(payload) {
			t.Errorf("expected %s, got %s", payload, msg)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("timed out waiting for broadcast message")
	}

	h.Unregister(client)
	time.Sleep(20 * time.Millisecond)

	if h.ClientCount() != 0 {
		t.Errorf("expected 0 clients after unregister, got %d", h.ClientCount())
	}
}

func TestWSHubBufferSaturation(t *testing.T) {
	h := NewWSHub()
	go h.Run()

	// Client with tiny buffer of 1
	client := &WSClient{
		hub:  h,
		send: make(chan []byte, 1),
	}

	h.Register(client)
	time.Sleep(20 * time.Millisecond)

	// Fill buffer
	h.Broadcast([]byte("PACKET_1"))
	time.Sleep(10 * time.Millisecond)

	// Saturated buffer: this second packet should be safely dropped without blocking
	h.Broadcast([]byte("PACKET_2"))

	if len(client.send) != 1 {
		t.Errorf("expected buffer to hold 1 packet, got %d", len(client.send))
	}
}
