package app

import (
	"context"
	"errors"
	"log/slog"

	"tailclip/internal/clipboard"
	"tailclip/internal/config"
	"tailclip/internal/event"
	"tailclip/internal/transport"
)

func Run(ctx context.Context, logger *slog.Logger, cfg config.Config) error {
	watcher := clipboard.NewWatcher(cfg.PollInterval)
	sender := transport.NewClient(cfg)
	var lastSentHash string

	logger.Info("tailclip agent started", "android_url", cfg.AndroidURL, "device_id", cfg.DeviceID)

	for {
		change, err := watcher.Next(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				logger.Info("tailclip agent stopped")
				return nil
			}

			return err
		}

		evt := event.NewClipboardEvent(change.Text, cfg.DeviceID, change.DetectedAt)
		if evt.ContentHash == lastSentHash {
			logger.Debug("skipping duplicate clipboard text", "content_hash", evt.ContentHash)
			continue
		}

		logger.Debug("sending clipboard update", "event_id", evt.ID, "content_hash", evt.ContentHash)
		if err := sender.SendClipboard(ctx, evt); err != nil {
			logger.Warn("clipboard send failed", "error", err, "event_id", evt.ID)
			continue
		}

		lastSentHash = evt.ContentHash
		logger.Info("clipboard synced", "event_id", evt.ID)
	}
}
