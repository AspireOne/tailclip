package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"tailclip/internal/clipboard"
	"tailclip/internal/config"
	"tailclip/internal/event"
	"tailclip/internal/transport"
)

func TestSyncStateSkipsInboundEchoUntilClipboardChanges(t *testing.T) {
	var state syncState
	hash := event.HashContent("hello")
	other := event.HashContent("world")

	state.markInboundApplied(hash)

	if skip := state.shouldSkipOutbound(hash); !skip {
		t.Fatal("expected first inbound echo to be skipped")
	}
	if skip := state.shouldSkipOutbound(hash); !skip {
		t.Fatal("expected repeated inbound echo to remain skipped")
	}
	if skip := state.shouldSkipOutbound(other); skip {
		t.Fatal("expected different clipboard content to pass through")
	}
	if skip := state.shouldSkipOutbound(hash); skip {
		t.Fatal("expected original content to be allowed again after clipboard changed")
	}
}

func TestSyncStateClearsPendingEchoAfterDifferentClipboardEvent(t *testing.T) {
	var state syncState
	alpha := event.HashContent("alpha")
	beta := event.HashContent("beta")

	state.markInboundApplied(alpha)

	if skip := state.shouldSkipOutbound(beta); skip {
		t.Fatal("expected different next clipboard event to pass through")
	}
	if skip := state.shouldSkipOutbound(alpha); skip {
		t.Fatal("expected pending echo hash to be cleared after clipboard changed")
	}
}

func TestSyncStateSkipsConsecutiveOutboundDuplicates(t *testing.T) {
	var state syncState
	hash := event.HashContent("hello")

	state.markSent(hash)

	if skip := state.shouldSkipOutbound(hash); !skip {
		t.Fatal("expected consecutive duplicate outbound content to be skipped")
	}
	if skip := state.shouldSkipOutbound(event.HashContent("world")); skip {
		t.Fatal("expected different outbound content to be sent")
	}
}

func TestDecideOutboundEventSkipsOversizedClipboardText(t *testing.T) {
	cfg := config.Default()
	cfg.DeviceID = "pc"
	cfg.MaxOutboundChars = 4

	change := clipboard.TextChange{
		Text:       "hello",
		DetectedAt: time.Unix(1700000000, 0),
	}

	decision := decideOutboundEvent(change, cfg, &syncState{})
	if decision.skipReason != outboundSkipOversized {
		t.Fatalf("expected oversized clipboard text to be skipped, got %q", decision.skipReason)
	}
	if decision.charCount != 5 {
		t.Fatalf("expected rune count to be reported, got %d", decision.charCount)
	}
}

func TestDecideOutboundEventAllowsTextAtLimit(t *testing.T) {
	cfg := config.Default()
	cfg.DeviceID = "pc"
	cfg.MaxOutboundChars = 5

	change := clipboard.TextChange{
		Text:       "hello",
		DetectedAt: time.Unix(1700000000, 0),
	}

	decision := decideOutboundEvent(change, cfg, &syncState{})
	if decision.skipReason != outboundSkipNone {
		t.Fatalf("expected clipboard text at limit to be sent, got %q", decision.skipReason)
	}
	if decision.evt.Content != "hello" {
		t.Fatalf("expected event content to round-trip, got %q", decision.evt.Content)
	}
}

func TestDecideOutboundEventCountsUnicodeCharacters(t *testing.T) {
	cfg := config.Default()
	cfg.DeviceID = "pc"
	cfg.MaxOutboundChars = 3

	change := clipboard.TextChange{
		Text:       "a🙂漢",
		DetectedAt: time.Unix(1700000000, 0),
	}

	decision := decideOutboundEvent(change, cfg, &syncState{})
	if decision.skipReason != outboundSkipNone {
		t.Fatalf("expected three-rune clipboard text to be allowed, got %q", decision.skipReason)
	}

	cfg.MaxOutboundChars = 2
	decision = decideOutboundEvent(change, cfg, &syncState{})
	if decision.skipReason != outboundSkipOversized {
		t.Fatalf("expected three-rune clipboard text to exceed limit 2, got %q", decision.skipReason)
	}
	if decision.charCount != 3 {
		t.Fatalf("expected Unicode rune count of 3, got %d", decision.charCount)
	}
}

func TestDecideOutboundEventClearsPendingEchoWhenOversizedTextDiffers(t *testing.T) {
	cfg := config.Default()
	cfg.DeviceID = "pc"
	cfg.MaxOutboundChars = 1

	var state syncState
	echoHash := event.HashContent("echo")
	state.markInboundApplied(echoHash)

	change := clipboard.TextChange{
		Text:       "large",
		DetectedAt: time.Unix(1700000000, 0),
	}

	decision := decideOutboundEvent(change, cfg, &state)
	if decision.skipReason != outboundSkipOversized {
		t.Fatalf("expected oversized clipboard text to be skipped, got %q", decision.skipReason)
	}
	if skip := state.shouldSkipOutbound(echoHash); skip {
		t.Fatal("expected oversized clipboard change to clear stale pending echo")
	}
}

func TestRunCancelsSiblingWorkerOnError(t *testing.T) {
	previousRunSenderLoop := runSenderLoop
	previousNewInboundServer := newInboundServer
	defer func() {
		runSenderLoop = previousRunSenderLoop
		newInboundServer = previousNewInboundServer
	}()

	serverCanceled := make(chan struct{})
	runSenderLoop = func(ctx context.Context, logger *slog.Logger, cfg config.Config, sender *transport.Client, state *syncState) error {
		return errors.New("sender failed")
	}
	newInboundServer = func(logger *slog.Logger, cfg config.Config, apply transport.ClipboardApplier) inboundServer {
		return fakeInboundServer{
			listenAndServe: func(ctx context.Context) error {
				<-ctx.Done()
				close(serverCanceled)
				return nil
			},
		}
	}

	cfg := config.Default()
	cfg.AndroidURL = "http://127.0.0.1/clipboard"
	cfg.WindowsListenAddr = ":8080"
	cfg.AuthToken = "token"
	cfg.DeviceID = "pc"

	err := Run(context.Background(), testLogger(), cfg)
	if err == nil || err.Error() != "sender failed" {
		t.Fatalf("expected sender error, got %v", err)
	}

	select {
	case <-serverCanceled:
	case <-time.After(time.Second):
		t.Fatal("expected sibling worker context to be canceled before Run returned")
	}
}

type fakeInboundServer struct {
	listenAndServe func(context.Context) error
}

func (s fakeInboundServer) ListenAndServe(ctx context.Context) error {
	return s.listenAndServe(ctx)
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
