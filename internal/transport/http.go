package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"tailclip/internal/config"
	"tailclip/internal/event"
)

type Client struct {
	httpClient *http.Client
	url        string
	authToken  string
}

func NewClient(cfg config.Config) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: cfg.HTTPTimeout},
		url:        cfg.AndroidURL,
		authToken:  cfg.AuthToken,
	}
}

func (c *Client) SendClipboard(ctx context.Context, evt event.ClipboardEvent) error {
	body, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshal clipboard event: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.authToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code: %s", resp.Status)
	}

	return nil
}
