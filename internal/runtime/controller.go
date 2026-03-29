package runtime

import (
	"context"
	"log/slog"
	"sync"

	"tailclip/internal/app"
	"tailclip/internal/config"
)

type State string

const (
	StateNeedsConfig State = "needs_config"
	StateRunning     State = "running"
	StateDisabled    State = "disabled"
	StateError       State = "error"
)

type Status struct {
	State   State
	Message string
}

type Controller struct {
	logger *slog.Logger
	runner func(context.Context, *slog.Logger, config.Config) error

	mu           sync.RWMutex
	cfg          config.Config
	hasCfg       bool
	status       Status
	cancel       context.CancelFunc
	running      bool
	nextRunID    uint64
	currentRunID uint64
	subs         []chan Status
}

func NewController(logger *slog.Logger) *Controller {
	c := &Controller{
		logger: logger,
		runner: app.Run,
		status: Status{
			State:   StateNeedsConfig,
			Message: "Set up Tailclip to start syncing.",
		},
	}
	return c
}

func (c *Controller) Apply(cfg config.Config) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.stopLocked()
	c.cfg = cfg
	c.hasCfg = true

	if !cfg.Enabled {
		c.setStatusLocked(Status{
			State:   StateDisabled,
			Message: "Syncing is disabled.",
		})
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	c.nextRunID++
	runID := c.nextRunID
	c.currentRunID = runID
	c.cancel = cancel
	c.running = true
	c.setStatusLocked(Status{
		State:   StateRunning,
		Message: "Syncing clipboard changes.",
	})

	go c.runLoop(ctx, cfg, runID)
}

func (c *Controller) Disable(reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.stopLocked()
	c.setStatusLocked(Status{
		State:   StateDisabled,
		Message: reason,
	})
}

func (c *Controller) SetNeedsConfig(message string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.stopLocked()
	c.hasCfg = false
	c.setStatusLocked(Status{
		State:   StateNeedsConfig,
		Message: message,
	})
}

func (c *Controller) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopLocked()
}

func (c *Controller) Status() Status {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

func (c *Controller) CurrentConfig() (config.Config, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cfg, c.hasCfg
}

func (c *Controller) Subscribe() <-chan Status {
	ch := make(chan Status, 1)

	c.mu.Lock()
	c.subs = append(c.subs, ch)
	current := c.status
	c.mu.Unlock()

	ch <- current
	return ch
}

func (c *Controller) runLoop(ctx context.Context, cfg config.Config, runID uint64) {
	if err := c.runner(ctx, c.logger, cfg); err != nil && ctx.Err() == nil {
		c.mu.Lock()
		defer c.mu.Unlock()

		if !c.finishRunLocked(runID) {
			return
		}
		c.setStatusLocked(Status{
			State:   StateError,
			Message: err.Error(),
		})
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.finishRunLocked(runID) {
		return
	}
	if c.hasCfg && !c.cfg.Enabled {
		c.setStatusLocked(Status{
			State:   StateDisabled,
			Message: "Syncing is disabled.",
		})
		return
	}

	if c.status.State == StateRunning {
		c.setStatusLocked(Status{
			State:   StateDisabled,
			Message: "Syncing stopped.",
		})
	}
}

func (c *Controller) SetRunner(run func(context.Context, *slog.Logger, config.Config) error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.runner = run
}

func (c *Controller) stopLocked() {
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	c.running = false
}

func (c *Controller) finishRunLocked(runID uint64) bool {
	if runID != c.currentRunID {
		return false
	}

	c.running = false
	c.cancel = nil
	c.currentRunID = 0
	return true
}

func (c *Controller) setStatusLocked(status Status) {
	c.status = status
	for _, ch := range c.subs {
		select {
		case ch <- status:
		default:
		}
	}
}
