//go:build windows

package clipboard

import (
	"sync/atomic"
	"syscall"
	"testing"
)

func TestEnsureWatcherWindowClassRegistersOnce(t *testing.T) {
	resetWatcherClassState(t)

	var calls atomic.Int32
	original := registerClassW
	registerClassW = func(wc *wndClass) (uintptr, uintptr, error) {
		calls.Add(1)
		return 1, 0, nil
	}
	t.Cleanup(func() {
		registerClassW = original
	})

	if err := ensureWatcherWindowClass(1); err != nil {
		t.Fatalf("first registration failed: %v", err)
	}
	if err := ensureWatcherWindowClass(1); err != nil {
		t.Fatalf("second registration failed: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected one registration attempt, got %d", got)
	}
}

func TestEnsureWatcherWindowClassRetriesAfterFailure(t *testing.T) {
	resetWatcherClassState(t)

	var calls atomic.Int32
	original := registerClassW
	registerClassW = func(wc *wndClass) (uintptr, uintptr, error) {
		if calls.Add(1) == 1 {
			return 0, 0, syscall.Errno(5)
		}
		return 1, 0, nil
	}
	t.Cleanup(func() {
		registerClassW = original
	})

	if err := ensureWatcherWindowClass(1); err == nil {
		t.Fatal("expected first registration to fail")
	}
	if err := ensureWatcherWindowClass(1); err != nil {
		t.Fatalf("expected retry to succeed, got %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected two registration attempts, got %d", got)
	}
}

func TestEnsureWatcherWindowClassAcceptsExistingClass(t *testing.T) {
	resetWatcherClassState(t)

	original := registerClassW
	registerClassW = func(wc *wndClass) (uintptr, uintptr, error) {
		return 0, 0, syscall.Errno(errClassExists)
	}
	t.Cleanup(func() {
		registerClassW = original
	})

	if err := ensureWatcherWindowClass(1); err != nil {
		t.Fatalf("expected existing class to be accepted, got %v", err)
	}
}

func resetWatcherClassState(t *testing.T) {
	t.Helper()

	watcherClassMu.Lock()
	watcherClassSet = false
	watcherClassMu.Unlock()
}
