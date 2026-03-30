package app

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"unicode/utf8"

	"tailclip/internal/clipboard"
	"tailclip/internal/config"
	"tailclip/internal/event"
	"tailclip/internal/transport"
)

type inboundServer interface {
	ListenAndServe(context.Context) error
}

var newInboundServer = func(logger *slog.Logger, cfg config.Config, apply transport.ClipboardApplier) inboundServer {
	return transport.NewServer(logger, cfg, apply)
}

var runSenderLoop = runSender

type outboundSkipReason string

const (
	outboundSkipNone      outboundSkipReason = ""
	outboundSkipDuplicate outboundSkipReason = "duplicate"
	outboundSkipOversized outboundSkipReason = "oversized"
)

type outboundDecision struct {
	evt         event.ClipboardEvent
	contentHash string
	charCount   int
	skipReason  outboundSkipReason
}

func Run(ctx context.Context, logger *slog.Logger, cfg config.Config) error {
	outboundEnabled := strings.TrimSpace(cfg.AndroidURL) != ""
	inboundEnabled := strings.TrimSpace(cfg.WindowsListenAddr) != ""
	if !outboundEnabled && !inboundEnabled {
		return errors.New("tailclip has no enabled sender or receiver endpoint")
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	sender := transport.NewClient(cfg)
	var state syncState

	logger.Info(
		"tailclip agent started",
		"android_url", cfg.AndroidURL,
		"windows_listen_addr", cfg.WindowsListenAddr,
		"device_id", cfg.DeviceID,
	)

	errCh := make(chan error, 2)
	var wg sync.WaitGroup

	if outboundEnabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := runSenderLoop(runCtx, logger, cfg, sender, &state); err != nil && !errors.Is(err, context.Canceled) {
				errCh <- err
			}
		}()
	}

	if inboundEnabled {
		server := newInboundServer(logger, cfg, func(callCtx context.Context, evt event.ClipboardEvent) error {
			return applyInbound(logger, evt, &state)
		})
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := server.ListenAndServe(runCtx); err != nil && !errors.Is(err, context.Canceled) {
				errCh <- err
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case err := <-errCh:
		cancel()
		<-done
		return err
	case <-ctx.Done():
		cancel()
		<-done
		logger.Info("tailclip agent stopped")
		return nil
	}
}

type syncState struct {
	mu              sync.Mutex
	lastSentHash    string
	pendingEchoHash string
}

func (s *syncState) shouldSkipOutbound(contentHash string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.pendingEchoHash != "" {
		if contentHash == s.pendingEchoHash {
			return true
		}
		s.pendingEchoHash = ""
	}

	return contentHash == s.lastSentHash
}

func (s *syncState) markSent(contentHash string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastSentHash = contentHash
}

func (s *syncState) markInboundApplied(contentHash string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pendingEchoHash = contentHash
}

func runSender(ctx context.Context, logger *slog.Logger, cfg config.Config, sender *transport.Client, state *syncState) error {
	watcher := clipboard.NewWatcher()

	for {
		change, err := watcher.Next(ctx)
		if err != nil {
			return err
		}

		decision := decideOutboundEvent(change, cfg, state)
		if decision.skipReason == outboundSkipDuplicate {
			logger.Debug("skipping duplicate clipboard text", "content_hash", decision.contentHash)
			continue
		}
		if decision.skipReason == outboundSkipOversized {
			logger.Info(
				"skipping oversized clipboard text",
				"content_hash", decision.contentHash,
				"content_chars", decision.charCount,
				"max_outbound_chars", cfg.MaxOutboundChars,
			)
			continue
		}

		evt := decision.evt
		logger.Debug("sending clipboard update", "event_id", evt.ID, "content_hash", evt.ContentHash)
		if err := sender.SendClipboard(ctx, evt); err != nil {
			logger.Warn("clipboard send failed", "error", err, "event_id", evt.ID)
			continue
		}

		state.markSent(evt.ContentHash)
		logger.Info("clipboard synced to android", "event_id", evt.ID)
	}
}

func decideOutboundEvent(change clipboard.TextChange, cfg config.Config, state *syncState) outboundDecision {
	contentHash := event.HashContent(change.Text)
	if state.shouldSkipOutbound(contentHash) {
		return outboundDecision{
			contentHash: contentHash,
			skipReason:  outboundSkipDuplicate,
		}
	}

	if cfg.MaxOutboundChars > 0 {
		charCount := utf8.RuneCountInString(change.Text)
		if charCount > cfg.MaxOutboundChars {
			return outboundDecision{
				contentHash: contentHash,
				charCount:   charCount,
				skipReason:  outboundSkipOversized,
			}
		}
	}

	evt := event.NewClipboardEvent(change.Text, cfg.DeviceID, change.DetectedAt)
	return outboundDecision{
		evt:         evt,
		contentHash: evt.ContentHash,
	}
}

func applyInbound(logger *slog.Logger, evt event.ClipboardEvent, state *syncState) error {
	if err := clipboard.SetText(evt.Content); err != nil {
		return err
	}

	state.markInboundApplied(evt.ContentHash)
	logger.Info("clipboard applied from android", "event_id", evt.ID, "source_device_id", evt.SourceDeviceID)
	return nil
}
