package runtime

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"tailclip/internal/config"
)

func TestApplyStartsRunnerForEnabledConfig(t *testing.T) {
	controller := NewController(testLogger())
	started := make(chan struct{}, 1)
	controller.SetRunner(func(ctx context.Context, logger *slog.Logger, cfg config.Config) error {
		started <- struct{}{}
		<-ctx.Done()
		return nil
	})

	cfg := config.Default()
	cfg.AndroidURL = "http://127.0.0.1/clipboard"
	cfg.AuthToken = "token"
	cfg.DeviceID = "pc"

	controller.Apply(cfg)
	defer controller.Stop()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("runner did not start")
	}

	if got := controller.Status(); got.State != StateRunning {
		t.Fatalf("expected running state, got %+v", got)
	}
}

func TestApplySkipsRunnerForDisabledConfig(t *testing.T) {
	controller := NewController(testLogger())
	called := make(chan struct{}, 1)
	controller.SetRunner(func(ctx context.Context, logger *slog.Logger, cfg config.Config) error {
		called <- struct{}{}
		return nil
	})

	cfg := config.Default()
	cfg.AndroidURL = "http://127.0.0.1/clipboard"
	cfg.AuthToken = "token"
	cfg.DeviceID = "pc"
	cfg.Enabled = false

	controller.Apply(cfg)
	defer controller.Stop()

	select {
	case <-called:
		t.Fatal("runner should not start for disabled config")
	case <-time.After(100 * time.Millisecond):
	}

	if got := controller.Status(); got.State != StateDisabled {
		t.Fatalf("expected disabled state, got %+v", got)
	}
}

func TestRunnerErrorSetsErrorState(t *testing.T) {
	controller := NewController(testLogger())
	controller.SetRunner(func(ctx context.Context, logger *slog.Logger, cfg config.Config) error {
		return context.DeadlineExceeded
	})

	cfg := config.Default()
	cfg.AndroidURL = "http://127.0.0.1/clipboard"
	cfg.AuthToken = "token"
	cfg.DeviceID = "pc"

	controller.Apply(cfg)
	defer controller.Stop()

	deadline := time.After(time.Second)
	for {
		if status := controller.Status(); status.State == StateError {
			return
		}

		select {
		case <-deadline:
			t.Fatal("expected error state")
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func TestApplyPreservesActiveRunStateWhenPreviousRunnerExits(t *testing.T) {
	controller := NewController(testLogger())

	var runCount atomic.Int32
	started := make(chan int32, 2)
	firstCanExit := make(chan struct{})
	firstExited := make(chan struct{})
	secondCanceled := make(chan struct{}, 1)

	controller.SetRunner(func(ctx context.Context, logger *slog.Logger, cfg config.Config) error {
		runID := runCount.Add(1)
		started <- runID

		switch runID {
		case 1:
			<-ctx.Done()
			<-firstCanExit
			close(firstExited)
			return nil
		case 2:
			<-ctx.Done()
			secondCanceled <- struct{}{}
			return nil
		default:
			return nil
		}
	})

	cfg := config.Default()
	cfg.AndroidURL = "http://127.0.0.1/clipboard"
	cfg.AuthToken = "token"
	cfg.DeviceID = "pc"

	controller.Apply(cfg)
	defer controller.Stop()

	waitForRunStart(t, started, 1)

	controller.Apply(cfg)
	waitForRunStart(t, started, 2)

	close(firstCanExit)

	select {
	case <-firstExited:
	case <-time.After(time.Second):
		t.Fatal("first runner did not exit")
	}

	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		controller.mu.RLock()
		cancel := controller.cancel
		status := controller.status
		controller.mu.RUnlock()

		if cancel == nil {
			t.Fatal("expected active cancel func to remain set for the newer run")
		}
		if status.State != StateRunning {
			t.Fatalf("expected controller to stay running, got %+v", status)
		}

		time.Sleep(10 * time.Millisecond)
	}

	controller.Disable("manual disable")

	select {
	case <-secondCanceled:
	case <-time.After(time.Second):
		t.Fatal("expected disable to cancel the active run")
	}
}

func TestSendTestClipUsesTester(t *testing.T) {
	controller := NewController(testLogger())

	var gotCfg config.Config
	var gotContent string
	controller.SetTester(func(ctx context.Context, logger *slog.Logger, cfg config.Config, content string) error {
		gotCfg = cfg
		gotContent = content
		return nil
	})

	cfg := config.Default()
	cfg.AndroidURL = "http://127.0.0.1/clipboard"
	cfg.AuthToken = "token"
	cfg.DeviceID = "pc"

	if err := controller.SendTestClip(context.Background(), cfg, "hello from test"); err != nil {
		t.Fatalf("SendTestClip returned error: %v", err)
	}

	if gotCfg.AndroidURL != cfg.AndroidURL {
		t.Fatalf("expected cfg to be passed through, got %+v", gotCfg)
	}
	if gotContent != "hello from test" {
		t.Fatalf("expected content to be passed through, got %q", gotContent)
	}
}

func TestSendTestClipValidatesConfig(t *testing.T) {
	controller := NewController(testLogger())

	cfg := config.Default()
	cfg.AuthToken = "token"
	cfg.DeviceID = "pc"

	err := controller.SendTestClip(context.Background(), cfg, "hello")
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "android_url") {
		t.Fatalf("expected android_url validation error, got %v", err)
	}
}

func waitForRunStart(t *testing.T, started <-chan int32, want int32) {
	t.Helper()

	select {
	case got := <-started:
		if got != want {
			t.Fatalf("expected run %d to start, got %d", want, got)
		}
	case <-time.After(time.Second):
		t.Fatalf("run %d did not start", want)
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
