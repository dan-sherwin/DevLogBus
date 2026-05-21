package configpath

import (
	"fmt"
	"os"
	"path/filepath"
)

func SettingsFile(appName string) (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	appConfigDir := filepath.Join(configDir, appName)
	if err := os.MkdirAll(appConfigDir, 0o755); err != nil {
		return "", fmt.Errorf("create config dir %s: %w", appConfigDir, err)
	}
	return filepath.Join(appConfigDir, "settings.db"), nil
}
