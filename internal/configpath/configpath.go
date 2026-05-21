package configpath

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

func SettingsFile(appName string) (string, error) {
	configDir, err := userConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	appConfigDir := filepath.Join(configDir, appName)
	if err := os.MkdirAll(appConfigDir, 0o755); err != nil {
		return "", fmt.Errorf("create config dir %s: %w", appConfigDir, err)
	}
	return filepath.Join(appConfigDir, "settings.db"), nil
}

func userConfigDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err == nil {
		return configDir, nil
	}
	if fallback, ok := serviceConfigDir(runtime.GOOS, os.Geteuid()); ok {
		return fallback, nil
	}
	return "", err
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
