package ingest

import (
	"io"
	"net/http"
	"strings"

	"github.com/luigieli/streaming/server/pkg/hub"
	"github.com/luigieli/streaming/utils/crypto"
)

type HTTPHandler struct {
	hub       *hub.WSHub
	streamKey string
}

func NewHTTPHandler(h *hub.WSHub, streamKey string) *HTTPHandler {
	return &HTTPHandler{
		hub:       h,
		streamKey: streamKey,
	}
}

func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	key := r.URL.Query().Get("key")
	if key == "" {
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			key = strings.TrimPrefix(auth, "Bearer ")
		}
	}

	if !crypto.ValidateKey(h.streamKey, key) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	const bufSize = 65536
	buf := make([]byte, bufSize)

	for {
		n, err := r.Body.Read(buf)
		if n > 0 {
			h.hub.Broadcast(buf[:n])
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			http.Error(w, "Error reading stream", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}
