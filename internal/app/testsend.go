package app

import (
	"context"
	"log/slog"
	"time"

	"tailclip/internal/config"
	"tailclip/internal/event"
	"tailclip/internal/transport"
)

func SendTestClipboard(ctx context.Context, logger *slog.Logger, cfg config.Config, content string) error {
	sender := transport.NewClient(cfg)
	evt := event.NewClipboardEvent(content, cfg.DeviceID, time.Now())

	logger.Info("sending test clipboard update", "event_id", evt.ID, "content_hash", evt.ContentHash)
	if err := sender.SendClipboard(ctx, evt); err != nil {
		return err
	}

	logger.Info("test clipboard update delivered", "event_id", evt.ID)
	return nil
}
