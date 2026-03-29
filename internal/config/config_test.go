package config

import (
	"testing"
	"time"
)

func TestDurationFromMSUsesFallback(t *testing.T) {
	got := durationFromMS(0, 3*time.Second)
	if got != 3*time.Second {
		t.Fatalf("expected fallback duration, got %v", got)
	}
}

func TestValidateRejectsMissingFields(t *testing.T) {
	cfg := Config{}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}
