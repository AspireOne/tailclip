package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultConfigDirName  = "tailclip"
	defaultConfigFileName = "config.json"
)

type Config struct {
	AndroidURL   string
	AuthToken    string
	DeviceID     string
	HTTPTimeout  time.Duration
	PollInterval time.Duration
	LogLevel     string
}

type fileConfig struct {
	AndroidURL     string `json:"android_url"`
	AuthToken      string `json:"auth_token"`
	DeviceID       string `json:"device_id"`
	HTTPTimeoutMS  int    `json:"http_timeout_ms"`
	PollIntervalMS int    `json:"poll_interval_ms"`
	LogLevel       string `json:"log_level"`
}

func Load(path string) (Config, error) {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return Config{}, err
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}

	var raw fileConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return Config{}, fmt.Errorf("parse config %q: %w", path, err)
	}

	cfg := Config{
		AndroidURL:   strings.TrimSpace(raw.AndroidURL),
		AuthToken:    strings.TrimSpace(raw.AuthToken),
		DeviceID:     strings.TrimSpace(raw.DeviceID),
		HTTPTimeout:  durationFromMS(raw.HTTPTimeoutMS, 3*time.Second),
		PollInterval: durationFromMS(raw.PollIntervalMS, 300*time.Millisecond),
		LogLevel:     strings.TrimSpace(raw.LogLevel),
	}

	if cfg.DeviceID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return Config{}, fmt.Errorf("resolve hostname for device_id: %w", err)
		}
		cfg.DeviceID = hostname
	}

	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func DefaultPath() (string, error) {
	appData, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve config dir: %w", err)
	}

	return filepath.Join(appData, defaultConfigDirName, defaultConfigFileName), nil
}

func (c Config) Validate() error {
	if c.AndroidURL == "" {
		return errors.New("config android_url is required")
	}
	if c.AuthToken == "" {
		return errors.New("config auth_token is required")
	}
	if c.DeviceID == "" {
		return errors.New("config device_id is required")
	}
	if c.HTTPTimeout <= 0 {
		return errors.New("config http_timeout_ms must be greater than zero")
	}
	if c.PollInterval <= 0 {
		return errors.New("config poll_interval_ms must be greater than zero")
	}
	return nil
}

func durationFromMS(value int, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return time.Duration(value) * time.Millisecond
}
