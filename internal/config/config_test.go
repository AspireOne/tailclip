package config

import (
	"os"
	"path/filepath"
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

func TestLoadDefaultsEnabledWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	data := []byte("{\n  \"android_url\": \"http://127.0.0.1/clipboard\",\n  \"auth_token\": \"token\",\n  \"device_id\": \"pc\"\n}\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if !cfg.Enabled {
		t.Fatal("expected enabled to default to true")
	}
	if cfg.WindowsListenAddr != "" {
		t.Fatalf("expected missing windows listen addr to stay disabled, got %q", cfg.WindowsListenAddr)
	}
	if cfg.MaxOutboundChars != 0 {
		t.Fatalf("expected missing max_outbound_chars to default to 0, got %d", cfg.MaxOutboundChars)
	}
}

func TestSaveCreatesConfigDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")

	cfg := Default()
	cfg.AndroidURL = "http://127.0.0.1/clipboard"
	cfg.AuthToken = "token"
	cfg.DeviceID = "pc"
	cfg.Enabled = false

	if err := Save(path, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	saved, err := Load(path)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}

	if saved.Enabled {
		t.Fatal("expected enabled=false after reload")
	}
	if saved.HTTPTimeout != 3*time.Second {
		t.Fatalf("expected default http timeout, got %v", saved.HTTPTimeout)
	}
}

func TestValidateAllowsReceiverOnlyConfig(t *testing.T) {
	cfg := Default()
	cfg.AndroidURL = ""
	cfg.WindowsListenAddr = ":8080"
	cfg.AuthToken = "token"
	cfg.DeviceID = "pc"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected receiver-only config to validate, got %v", err)
	}
}

func TestDefaultLeavesInboundDisabled(t *testing.T) {
	cfg := Default()
	if cfg.WindowsListenAddr != "" {
		t.Fatalf("expected default windows listen addr to be disabled, got %q", cfg.WindowsListenAddr)
	}
}

func TestSavePreservesDisabledInboundListener(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")

	cfg := Default()
	cfg.AndroidURL = "http://127.0.0.1/clipboard"
	cfg.WindowsListenAddr = ""
	cfg.AuthToken = "token"
	cfg.DeviceID = "pc"

	if err := Save(path, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	saved, err := Load(path)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}

	if saved.WindowsListenAddr != "" {
		t.Fatalf("expected disabled inbound listener to round-trip, got %q", saved.WindowsListenAddr)
	}
}

func TestValidateRejectsNegativeMaxOutboundChars(t *testing.T) {
	cfg := Default()
	cfg.AndroidURL = "http://127.0.0.1/clipboard"
	cfg.AuthToken = "token"
	cfg.DeviceID = "pc"
	cfg.MaxOutboundChars = -1

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected negative max_outbound_chars to be rejected")
	}
}

func TestSaveRoundTripsMaxOutboundChars(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")

	cfg := Default()
	cfg.AndroidURL = "http://127.0.0.1/clipboard"
	cfg.AuthToken = "token"
	cfg.DeviceID = "pc"
	cfg.MaxOutboundChars = 200

	if err := Save(path, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	saved, err := Load(path)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}

	if saved.MaxOutboundChars != 200 {
		t.Fatalf("expected max_outbound_chars to round-trip, got %d", saved.MaxOutboundChars)
	}
}
