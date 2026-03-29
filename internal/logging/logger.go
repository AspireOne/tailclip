package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

func New(level string) (*slog.Logger, io.Closer, error) {
	path, err := DefaultPath()
	if err != nil {
		return nil, nil, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, nil, fmt.Errorf("create log dir: %w", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("open log file: %w", err)
	}

	handler := slog.NewTextHandler(newLogWriter(file, os.Stdout), &slog.HandlerOptions{
		Level: parseLevel(level),
	})

	return slog.New(handler), file, nil
}

func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve config dir: %w", err)
	}

	return filepath.Join(dir, "tailclip", "logs", "tailclip.log"), nil
}

func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func newLogWriter(primary io.Writer, mirrors ...io.Writer) io.Writer {
	return logWriter{
		primary: primary,
		mirrors: mirrors,
	}
}

type logWriter struct {
	primary io.Writer
	mirrors []io.Writer
}

func (w logWriter) Write(p []byte) (int, error) {
	n, err := w.primary.Write(p)
	if err != nil {
		return n, err
	}
	if n != len(p) {
		return n, io.ErrShortWrite
	}

	for _, mirror := range w.mirrors {
		if mirror == nil {
			continue
		}
		_, _ = mirror.Write(p)
	}

	return n, nil
}
