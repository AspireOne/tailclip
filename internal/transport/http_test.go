package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"tailclip/internal/config"
	"tailclip/internal/event"
)

func TestSendClipboard(t *testing.T) {
	var gotAuth string
	var got event.ClipboardEvent

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(config.Config{
		AndroidURL:  server.URL,
		AuthToken:   "secret",
		HTTPTimeout: time.Second,
	})

	evt := event.NewClipboardEvent("hello", "pc", time.Now())
	if err := client.SendClipboard(context.Background(), evt); err != nil {
		t.Fatalf("SendClipboard returned error: %v", err)
	}

	if gotAuth != "Bearer secret" {
		t.Fatalf("expected auth header, got %q", gotAuth)
	}
	if got.Content != "hello" {
		t.Fatalf("expected content to round-trip, got %q", got.Content)
	}
}
