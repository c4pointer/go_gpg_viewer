package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Settings represents the application configuration
type Settings struct {
	PasswordStorePath string  `json:"password_store_path"`
	DefaultRecipient  string  `json:"default_recipient"`
	AutoCommit        bool    `json:"auto_commit"`
	ShowNotifications bool    `json:"show_notifications"`
	Theme             string  `json:"theme"`
	WindowWidth       int     `json:"window_width"`
	WindowHeight      int     `json:"window_height"`
	SplitOffset       float64 `json:"split_offset"`
}

// File and directory permission constants for the config file. The file
// may include a default GPG recipient and the password-store path, so it
// is treated as user-private (0600 / 0700) rather than world-readable.
const (
	configFileMode os.FileMode = 0600
	configDirMode  os.FileMode = 0700
)

// recipientEmailRe matches the email form of a GPG recipient. The pattern is
// deliberately permissive — gpg itself does the real lookup; we just want to
// catch obvious typos like trailing spaces or accidental angle brackets.
var recipientEmailRe = regexp.MustCompile(`^[^@\s<>]+@[^@\s<>]+\.[^@\s<>]+$`)

// recipientKeyIDRe matches a short / long / fingerprint key id, optionally
// prefixed with "0x".
var recipientKeyIDRe = regexp.MustCompile(`^(0x)?[0-9A-Fa-f]{8,40}$`)

// IsValidRecipient reports whether s looks like a plausible GPG recipient:
// either an email address or a hex key id / fingerprint. An empty string is
// also accepted — it just means "no default recipient configured". Returning
// false here does not guarantee gpg will reject the value (it can still
// accept user IDs from the keyring), but it does flag clearly-malformed
// inputs before they are persisted.
func IsValidRecipient(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	return recipientEmailRe.MatchString(s) || recipientKeyIDRe.MatchString(s)
}

// DefaultSettings returns the default configuration
func DefaultSettings() *Settings {
	return &Settings{
		PasswordStorePath: "", // Will be set to ~/.password-store by default
		DefaultRecipient:  "",
		AutoCommit:        true,
		ShowNotifications: true,
		Theme:             "light",
		WindowWidth:       800,
		WindowHeight:      600,
		SplitOffset:       0.3,
	}
}

// LoadSettings loads settings from the configuration file
func LoadSettings() (*Settings, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get config path: %w", err)
	}

	// If config file doesn't exist, return default settings
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		settings := DefaultSettings()
		// Save default settings
		if err := SaveSettings(settings); err != nil {
			return nil, fmt.Errorf("failed to save default settings: %w", err)
		}
		return settings, nil
	}

	// Read existing config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &settings, nil
}

// SaveSettings writes the settings to disk atomically. The file is written
// to a sibling temp file with 0600 permissions, then os.Rename'd into place,
// so a crash mid-write cannot leave a half-truncated JSON document behind.
func SaveSettings(settings *Settings) error {
	configPath, err := getConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}

	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, configDirMode); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	tmpFile, err := os.CreateTemp(configDir, "settings-*.json.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp config file: %w", err)
	}
	tmpPath := tmpFile.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := tmpFile.Chmod(configFileMode); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to chmod temp config file: %w", err)
	}
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write temp config file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp config file: %w", err)
	}

	if err := os.Rename(tmpPath, configPath); err != nil {
		return fmt.Errorf("failed to rename temp config file: %w", err)
	}
	cleanup = false
	return nil
}

// getConfigPath returns the path to the configuration file. It honours
// $XDG_CONFIG_HOME (via os.UserConfigDir), falling back to ~/.config on
// Linux or the platform-native location on macOS / Windows.
func getConfigPath() (string, error) {
	configBase, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user config directory: %w", err)
	}
	return filepath.Join(configBase, "gpg_viewer", "settings.json"), nil
}

// UpdateSettings updates specific settings and saves them
func UpdateSettings(updates map[string]interface{}) error {
	settings, err := LoadSettings()
	if err != nil {
		return fmt.Errorf("failed to load current settings: %w", err)
	}

	// Apply updates
	for key, value := range updates {
		switch key {
		case "password_store_path":
			if str, ok := value.(string); ok {
				settings.PasswordStorePath = str
			}
		case "default_recipient":
			if str, ok := value.(string); ok {
				settings.DefaultRecipient = str
			}
		case "auto_commit":
			if b, ok := value.(bool); ok {
				settings.AutoCommit = b
			}
		case "show_notifications":
			if b, ok := value.(bool); ok {
				settings.ShowNotifications = b
			}
		case "theme":
			if str, ok := value.(string); ok {
				settings.Theme = str
			}
		case "window_width":
			if i, ok := value.(int); ok {
				settings.WindowWidth = i
			}
		case "window_height":
			if i, ok := value.(int); ok {
				settings.WindowHeight = i
			}
		case "split_offset":
			if f, ok := value.(float64); ok {
				settings.SplitOffset = f
			}
		}
	}

	// Save updated settings
	return SaveSettings(settings)
}
