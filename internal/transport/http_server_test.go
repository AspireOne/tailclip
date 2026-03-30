package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"tailclip/internal/config"
	"tailclip/internal/event"
)

func TestServerAcceptsInboundShare(t *testing.T) {
	var got event.ClipboardEvent
	server := NewServer(testLogger(), config.Config{
		WindowsListenAddr: ":8080",
		AuthToken:         "secret",
		DeviceID:          "pc",
	}, func(ctx context.Context, evt event.ClipboardEvent) error {
		got = evt
		return nil
	})
	server.now = func() time.Time {
		return time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, sharePath, bodyReader(t, map[string]any{
		"content": "hello",
	}))
	req.Header.Set("Authorization", "Bearer secret")

	server.handleShare(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got.Content != "hello" {
		t.Fatalf("expected content to round-trip, got %q", got.Content)
	}
	if got.ContentHash != event.HashContent("hello") {
		t.Fatalf("expected content hash to be populated, got %q", got.ContentHash)
	}
	if got.ID == "" {
		t.Fatal("expected event ID to be populated")
	}
}

func TestServerPreservesInboundWhitespace(t *testing.T) {
	var got event.ClipboardEvent
	content := "  hello\n"
	server := NewServer(testLogger(), config.Config{
		WindowsListenAddr: ":8080",
		AuthToken:         "secret",
		DeviceID:          "pc",
	}, func(ctx context.Context, evt event.ClipboardEvent) error {
		got = evt
		return nil
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, sharePath, bodyReader(t, map[string]any{
		"content": content,
	}))
	req.Header.Set("Authorization", "Bearer secret")

	server.handleShare(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got.Content != content {
		t.Fatalf("expected content to preserve whitespace, got %q", got.Content)
	}
	if got.ContentHash != event.HashContent(content) {
		t.Fatalf("expected content hash for original content, got %q", got.ContentHash)
	}
}

func TestServerRecomputesInboundContentHash(t *testing.T) {
	var got event.ClipboardEvent
	content := "hello"
	server := NewServer(testLogger(), config.Config{
		WindowsListenAddr: ":8080",
		AuthToken:         "secret",
		DeviceID:          "pc",
	}, func(ctx context.Context, evt event.ClipboardEvent) error {
		got = evt
		return nil
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, sharePath, bodyReader(t, map[string]any{
		"content":      content,
		"content_hash": "sha256:stale",
	}))
	req.Header.Set("Authorization", "Bearer secret")

	server.handleShare(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got.ContentHash != event.HashContent(content) {
		t.Fatalf("expected stale inbound hash to be replaced, got %q", got.ContentHash)
	}
}

func TestServerRejectsUnauthorizedRequests(t *testing.T) {
	server := NewServer(testLogger(), config.Config{
		WindowsListenAddr: ":8080",
		AuthToken:         "secret",
		DeviceID:          "pc",
	}, func(ctx context.Context, evt event.ClipboardEvent) error {
		t.Fatal("apply should not be called")
		return nil
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, sharePath, bodyReader(t, map[string]any{
		"content": "hello",
	}))

	server.handleShare(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestServerRejectsEmptyContent(t *testing.T) {
	server := NewServer(testLogger(), config.Config{
		WindowsListenAddr: ":8080",
		AuthToken:         "secret",
		DeviceID:          "pc",
	}, func(ctx context.Context, evt event.ClipboardEvent) error {
		t.Fatal("apply should not be called")
		return nil
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, sharePath, bodyReader(t, map[string]any{
		"content": "   ",
	}))
	req.Header.Set("Authorization", "Bearer secret")

	server.handleShare(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestServerRejectsWrongMethod(t *testing.T) {
	server := NewServer(testLogger(), config.Config{
		WindowsListenAddr: ":8080",
		AuthToken:         "secret",
		DeviceID:          "pc",
	}, func(ctx context.Context, evt event.ClipboardEvent) error {
		t.Fatal("apply should not be called")
		return nil
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, sharePath, nil)
	req.Header.Set("Authorization", "Bearer secret")

	server.handleShare(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func bodyReader(t *testing.T, body map[string]any) io.Reader {
	t.Helper()

	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	return bytes.NewReader(data)
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
