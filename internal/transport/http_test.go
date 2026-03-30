package transport

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"tailclip/internal/config"
	"tailclip/internal/event"
)

func TestClientSendsValidRequest(t *testing.T) {
	evt := event.NewClipboardEvent("hello", "pc", time.Now())
	token := "secret-token"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json, got %s", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Authorization") != "Bearer "+token {
			t.Errorf("expected Bearer token, got %s", r.Header.Get("Authorization"))
		}

		body, _ := io.ReadAll(r.Body)
		var received event.ClipboardEvent
		if err := json.Unmarshal(body, &received); err != nil {
			t.Errorf("failed to unmarshal body: %v", err)
		}
		if received.Content != evt.Content {
			t.Errorf("expected content %q, got %q", evt.Content, received.Content)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config.Default()
	cfg.AndroidURL = server.URL
	cfg.AuthToken = token
	client := NewClient(cfg)

	err := client.SendClipboard(context.Background(), evt)
	if err != nil {
		t.Fatalf("SendClipboard failed: %v", err)
	}
}

func TestClientHandlesErrorStatusCodes(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"unauthorized", http.StatusUnauthorized},
		{"not found", http.StatusNotFound},
		{"server error", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			cfg := config.Default()
			cfg.AndroidURL = server.URL
			client := NewClient(cfg)

			err := client.SendClipboard(context.Background(), event.ClipboardEvent{})
			if err == nil {
				t.Fatal("expected error for non-2xx status code")
			}
		})
	}
}

func TestClientRespectsContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config.Default()
	cfg.AndroidURL = server.URL
	client := NewClient(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := client.SendClipboard(ctx, event.ClipboardEvent{})
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}

func TestClientRespectsTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config.Default()
	cfg.AndroidURL = server.URL
	cfg.HTTPTimeout = 10 * time.Millisecond
	client := NewClient(cfg)

	err := client.SendClipboard(context.Background(), event.ClipboardEvent{})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
