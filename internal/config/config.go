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
	defaultHTTPTimeout    = 3 * time.Second
	defaultPollInterval   = 300 * time.Millisecond
)

type Config struct {
	AndroidURL        string
	WindowsListenAddr string
	AuthToken         string
	DeviceID          string
	HTTPTimeout       time.Duration
	PollInterval      time.Duration
	LogLevel          string
	Enabled           bool
}

type fileConfig struct {
	AndroidURL        string `json:"android_url"`
	WindowsListenAddr string `json:"windows_listen_addr"`
	AuthToken         string `json:"auth_token"`
	DeviceID          string `json:"device_id"`
	HTTPTimeoutMS     int    `json:"http_timeout_ms"`
	PollIntervalMS    int    `json:"poll_interval_ms"`
	LogLevel          string `json:"log_level"`
	Enabled           *bool  `json:"enabled,omitempty"`
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

	cfg, err := normalize(raw)
	if err != nil {
		return Config{}, err
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func Save(path string, cfg Config) error {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return err
		}
	}

	cfg, err := normalizeFileConfig(toFileConfig(cfg))
	if err != nil {
		return err
	}

	if err := cfg.Validate(); err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config dir %q: %w", dir, err)
	}

	data, err := json.MarshalIndent(toFileConfig(cfg), "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config %q: %w", path, err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config %q: %w", path, err)
	}

	return nil
}

func Default() Config {
	return Config{
		HTTPTimeout:  defaultHTTPTimeout,
		PollInterval: defaultPollInterval,
		LogLevel:     "info",
		Enabled:      true,
	}
}

func DefaultPath() (string, error) {
	appData, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve config dir: %w", err)
	}

	return filepath.Join(appData, defaultConfigDirName, defaultConfigFileName), nil
}

func (c Config) Validate() error {
	if c.AndroidURL == "" && c.WindowsListenAddr == "" {
		return errors.New("config must set android_url, windows_listen_addr, or both")
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

func normalize(raw fileConfig) (Config, error) {
	cfg := Default()
	cfg.AndroidURL = strings.TrimSpace(raw.AndroidURL)
	cfg.WindowsListenAddr = strings.TrimSpace(raw.WindowsListenAddr)
	cfg.AuthToken = strings.TrimSpace(raw.AuthToken)
	cfg.DeviceID = strings.TrimSpace(raw.DeviceID)
	cfg.HTTPTimeout = durationFromMS(raw.HTTPTimeoutMS, defaultHTTPTimeout)
	cfg.PollInterval = durationFromMS(raw.PollIntervalMS, defaultPollInterval)
	cfg.LogLevel = strings.TrimSpace(raw.LogLevel)
	if raw.Enabled != nil {
		cfg.Enabled = *raw.Enabled
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

	return cfg, nil
}

func normalizeFileConfig(raw fileConfig) (Config, error) {
	return normalize(raw)
}

func toFileConfig(cfg Config) fileConfig {
	enabled := cfg.Enabled
	return fileConfig{
		AndroidURL:        cfg.AndroidURL,
		WindowsListenAddr: cfg.WindowsListenAddr,
		AuthToken:         cfg.AuthToken,
		DeviceID:          cfg.DeviceID,
		HTTPTimeoutMS:     int(cfg.HTTPTimeout / time.Millisecond),
		PollIntervalMS:    int(cfg.PollInterval / time.Millisecond),
		LogLevel:          cfg.LogLevel,
		Enabled:           &enabled,
	}
}
