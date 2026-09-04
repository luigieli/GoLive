package pipeline

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type Sender struct {
	serverURL string
	streamKey string
	client    *http.Client
}

func NewSender(serverURL, streamKey string) *Sender {
	return &Sender{
		serverURL: serverURL,
		streamKey: streamKey,
		client:    &http.Client{},
	}
}

func (s *Sender) Send(ctx context.Context, streamReader io.Reader) error {
	u, err := url.Parse(s.serverURL)
	if err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}

	q := u.Query()
	if s.streamKey != "" {
		q.Set("key", s.streamKey)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), streamReader)
	if err != nil {
		return fmt.Errorf("failed to create ingest request: %w", err)
	}
	req.Header.Set("Content-Type", "video/mp2t")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to stream to server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("server rejected ingest stream with status: %d", resp.StatusCode)
	}

	return nil
}
