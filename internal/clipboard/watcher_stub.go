//go:build !windows

package clipboard

import (
	"context"
	"time"
)

type TextChange struct {
	Text       string
	DetectedAt time.Time
}

type Watcher struct{}

func NewWatcher() *Watcher {
	return &Watcher{}
}

func (w *Watcher) Next(ctx context.Context) (TextChange, error) {
	<-ctx.Done()
	return TextChange{}, ctx.Err()
}
