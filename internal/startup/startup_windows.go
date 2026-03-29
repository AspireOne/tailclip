//go:build windows

package startup

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
	valueName  = "Tailclip"
)

func Enabled() (bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return false, nil
		}
		return false, fmt.Errorf("open startup key: %w", err)
	}
	defer key.Close()

	value, _, err := key.GetStringValue(valueName)
	if err != nil {
		if err == registry.ErrNotExist {
			return false, nil
		}
		return false, fmt.Errorf("read startup value: %w", err)
	}

	return strings.TrimSpace(value) != "", nil
}

func SetEnabled(enabled bool, configPath string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return fmt.Errorf("open startup key: %w", err)
	}
	defer key.Close()

	if !enabled {
		if err := key.DeleteValue(valueName); err != nil && err != registry.ErrNotExist {
			return fmt.Errorf("delete startup value: %w", err)
		}
		return nil
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}

	command := fmt.Sprintf("%q -config %q", exePath, configPath)
	if err := key.SetStringValue(valueName, command); err != nil {
		return fmt.Errorf("write startup value: %w", err)
	}

	return nil
}
