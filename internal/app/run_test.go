package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"tailclip/internal/config"
	"tailclip/internal/event"
	"tailclip/internal/transport"
)

func TestSyncStateSkipsOnlyImmediateEcho(t *testing.T) {
	var state syncState
	hash := event.HashContent("hello")

	state.markInboundApplied(hash)

	if skip := state.shouldSkipOutbound(hash); !skip {
		t.Fatal("expected immediate echo to be skipped")
	}
	if skip := state.shouldSkipOutbound(hash); skip {
		t.Fatal("expected later copy of the same content to be allowed")
	}
}

func TestSyncStateClearsPendingEchoAfterNextOutboundEvent(t *testing.T) {
	var state syncState
	alpha := event.HashContent("alpha")
	beta := event.HashContent("beta")

	state.markInboundApplied(alpha)

	if skip := state.shouldSkipOutbound(beta); skip {
		t.Fatal("expected different next clipboard event to pass through")
	}
	if skip := state.shouldSkipOutbound(alpha); skip {
		t.Fatal("expected pending echo hash to be cleared after one outbound event")
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
