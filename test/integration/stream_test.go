package integration

import (
	"bytes"
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/luigieli/streaming/client/pkg/pipeline"
	serverHttp "github.com/luigieli/streaming/server/pkg/http"
	"github.com/luigieli/streaming/server/pkg/hub"
	"github.com/luigieli/streaming/server/pkg/ingest"
)

func TestEndToEndClientServerStream(t *testing.T) {
	streamKey := "integration-test-secret-123"

	// 1. Initialize Server Hubs & Ingest
	wsHub := hub.NewWSHub()
	go wsHub.Run()

	httpIngest := ingest.NewHTTPHandler(wsHub, streamKey)
	server := serverHttp.NewServer(0, wsHub, nil, httpIngest, "")

	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	// 2. Connect Mock Viewer via WebSocket
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	viewerConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Viewer failed to connect to WebSocket: %v", err)
	}
	defer viewerConn.Close()

	// 3. Prepare Dummy MPEG-TS Stream
	testPayload := []byte("MPEGTS_INTEGRATION_TEST_BURST_PAYLOAD_CHUNK_1234567890")
	streamReader := bytes.NewReader(testPayload)

	// 4. Client Sender streams payload to Server Ingest
	sender := pipeline.NewSender(ts.URL+"/api/publish", streamKey)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = sender.Send(ctx, streamReader)
	if err != nil {
		t.Fatalf("Client sender failed to stream: %v", err)
	}

	// 5. Verify Viewer receives streamed payload
	_ = viewerConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	messageType, receivedData, err := viewerConn.ReadMessage()
	if err != nil {
		t.Fatalf("Viewer failed to receive message: %v", err)
	}

	if messageType != websocket.BinaryMessage {
		t.Errorf("expected BinaryMessage, got %d", messageType)
	}

	if !bytes.Equal(receivedData, testPayload) {
		t.Errorf("data mismatch:\nexpected: %s\ngot:      %s", testPayload, receivedData)
	}
}
