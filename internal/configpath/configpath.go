package configpath

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

func SettingsFile(appName string) (string, error) {
	configDir, err := defaultConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve settings config dir: %w", err)
	}
	appConfigDir := filepath.Join(configDir, appName)
	if err := os.MkdirAll(appConfigDir, 0o755); err != nil {
		return "", fmt.Errorf("create config dir %s: %w", appConfigDir, err)
	}
	return filepath.Join(appConfigDir, "settings.db"), nil
}

func defaultConfigDir() (string, error) {
	return configDir(runtime.GOOS, os.Geteuid(), os.UserConfigDir)
}

func configDir(goos string, euid int, userConfigDir func() (string, error)) (string, error) {
	if fallback, ok := serviceConfigDir(goos, euid); ok {
		return fallback, nil
	}
	return userConfigDir()
}

func serviceConfigDir(goos string, euid int) (string, bool) {
	if euid != 0 {
		return "", false
	}
	switch goos {
	case "darwin":
		return "/Library/Application Support", true
	case "linux":
		return "/var/lib", true
	default:
		return "", false
	}
}
