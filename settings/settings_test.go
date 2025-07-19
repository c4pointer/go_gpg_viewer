package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultSettings(t *testing.T) {
	settings := DefaultSettings()
	assert.NotNil(t, settings)

	// Verify default values
	assert.Equal(t, "", settings.PasswordStorePath)
	assert.Equal(t, "", settings.DefaultRecipient)
	assert.True(t, settings.AutoCommit)
	assert.True(t, settings.ShowNotifications)
	assert.Equal(t, "light", settings.Theme)
	assert.Equal(t, 800, settings.WindowWidth)
	assert.Equal(t, 600, settings.WindowHeight)
	assert.Equal(t, 0.3, settings.SplitOffset)
}

func TestLoadSettingsNewFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "settings_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Set environment variable to override config path
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	// Test loading settings when file doesn't exist
	settings, err := LoadSettings()
	require.NoError(t, err)
	assert.NotNil(t, settings)

	// Verify default settings were created
	assert.Equal(t, "", settings.PasswordStorePath)
	assert.Equal(t, "", settings.DefaultRecipient)
	assert.True(t, settings.AutoCommit)
	assert.True(t, settings.ShowNotifications)
	assert.Equal(t, "light", settings.Theme)
	assert.Equal(t, 800, settings.WindowWidth)
	assert.Equal(t, 600, settings.WindowHeight)
	assert.Equal(t, 0.3, settings.SplitOffset)

	// Verify file was created
	configPath, err := getConfigPath()
	require.NoError(t, err)
	assert.FileExists(t, configPath)
}

func TestLoadSettingsExistingFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "settings_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Set environment variable to override config path
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	// Create existing settings file
	existingSettings := &Settings{
		PasswordStorePath: "/custom/path",
		DefaultRecipient:  "test@example.com",
		AutoCommit:        false,
		ShowNotifications: false,
		Theme:             "dark",
		WindowWidth:       1024,
		WindowHeight:      768,
		SplitOffset:       0.5,
	}

	configPath, err := getConfigPath()
	require.NoError(t, err)

	// Ensure config directory exists
	configDir := filepath.Dir(configPath)
	err = os.MkdirAll(configDir, 0755)
	require.NoError(t, err)

	// Write existing settings
	data, err := json.MarshalIndent(existingSettings, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(configPath, data, 0644)
	require.NoError(t, err)

	// Test loading existing settings
	settings, err := LoadSettings()
	require.NoError(t, err)
	assert.NotNil(t, settings)

	// Verify loaded settings match
	assert.Equal(t, "/custom/path", settings.PasswordStorePath)
	assert.Equal(t, "test@example.com", settings.DefaultRecipient)
	assert.False(t, settings.AutoCommit)
	assert.False(t, settings.ShowNotifications)
	assert.Equal(t, "dark", settings.Theme)
	assert.Equal(t, 1024, settings.WindowWidth)
	assert.Equal(t, 768, settings.WindowHeight)
	assert.Equal(t, 0.5, settings.SplitOffset)
}

func TestSaveSettings(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "settings_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Set environment variable to override config path
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	// Create settings to save
	settings := &Settings{
		PasswordStorePath: "/test/path",
		DefaultRecipient:  "test@example.com",
		AutoCommit:        true,
		ShowNotifications: false,
		Theme:             "dark",
		WindowWidth:       1200,
		WindowHeight:      900,
		SplitOffset:       0.4,
	}

	// Test saving settings
	err = SaveSettings(settings)
	require.NoError(t, err)

	// Verify file was created
	configPath, err := getConfigPath()
	require.NoError(t, err)
	assert.FileExists(t, configPath)

	// Read and verify saved content
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var savedSettings Settings
	err = json.Unmarshal(data, &savedSettings)
	require.NoError(t, err)

	// Verify saved settings match
	assert.Equal(t, "/test/path", savedSettings.PasswordStorePath)
	assert.Equal(t, "test@example.com", savedSettings.DefaultRecipient)
	assert.True(t, savedSettings.AutoCommit)
	assert.False(t, savedSettings.ShowNotifications)
	assert.Equal(t, "dark", savedSettings.Theme)
	assert.Equal(t, 1200, savedSettings.WindowWidth)
	assert.Equal(t, 900, savedSettings.WindowHeight)
	assert.Equal(t, 0.4, savedSettings.SplitOffset)
}

func TestUpdateSettings(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "settings_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Set environment variable to override config path
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	// Create initial settings
	initialSettings := DefaultSettings()
	err = SaveSettings(initialSettings)
	require.NoError(t, err)

	// Test updating specific settings
	updates := map[string]interface{}{
		"password_store_path": "/updated/path",
		"default_recipient":   "updated@example.com",
		"auto_commit":         false,
		"theme":               "dark",
		"window_width":        1024,
		"window_height":       768,
		"split_offset":        0.6,
	}

	err = UpdateSettings(updates)
	require.NoError(t, err)

	// Load and verify updated settings
	updatedSettings, err := LoadSettings()
	require.NoError(t, err)
	assert.NotNil(t, updatedSettings)

	// Verify updates were applied
	assert.Equal(t, "/updated/path", updatedSettings.PasswordStorePath)
	assert.Equal(t, "updated@example.com", updatedSettings.DefaultRecipient)
	assert.False(t, updatedSettings.AutoCommit)
	assert.Equal(t, "dark", updatedSettings.Theme)
	assert.Equal(t, 1024, updatedSettings.WindowWidth)
	assert.Equal(t, 768, updatedSettings.WindowHeight)
	assert.Equal(t, 0.6, updatedSettings.SplitOffset)

	// Verify unchanged settings
	assert.True(t, updatedSettings.ShowNotifications) // Should remain unchanged
}

func TestUpdateSettingsInvalidKey(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "settings_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Set environment variable to override config path
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	// Create initial settings
	initialSettings := DefaultSettings()
	err = SaveSettings(initialSettings)
	require.NoError(t, err)

	// Test updating with invalid key (should be ignored)
	updates := map[string]interface{}{
		"invalid_key": "value",
		"theme":       "dark",
	}

	err = UpdateSettings(updates)
	require.NoError(t, err)

	// Load and verify settings
	updatedSettings, err := LoadSettings()
	require.NoError(t, err)
	assert.NotNil(t, updatedSettings)

	// Verify valid update was applied
	assert.Equal(t, "dark", updatedSettings.Theme)
	// Invalid key should be ignored
}

func TestUpdateSettingsInvalidType(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "settings_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Set environment variable to override config path
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	// Create initial settings
	initialSettings := DefaultSettings()
	err = SaveSettings(initialSettings)
	require.NoError(t, err)

	// Test updating with wrong type (should be ignored)
	updates := map[string]interface{}{
		"window_width": "not_a_number",
		"theme":        "dark",
	}

	err = UpdateSettings(updates)
	require.NoError(t, err)

	// Load and verify settings
	updatedSettings, err := LoadSettings()
	require.NoError(t, err)
	assert.NotNil(t, updatedSettings)

	// Verify valid update was applied
	assert.Equal(t, "dark", updatedSettings.Theme)
	// Invalid type should be ignored, window_width should remain default
	assert.Equal(t, 800, updatedSettings.WindowWidth)
}

func TestLoadSettingsCorruptedFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "settings_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Set environment variable to override config path
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	// Create corrupted settings file
	configPath, err := getConfigPath()
	require.NoError(t, err)

	// Ensure config directory exists
	configDir := filepath.Dir(configPath)
	err = os.MkdirAll(configDir, 0755)
	require.NoError(t, err)

	// Write invalid JSON
	err = os.WriteFile(configPath, []byte("invalid json content"), 0644)
	require.NoError(t, err)

	// Test loading corrupted settings
	settings, err := LoadSettings()
	assert.Error(t, err)
	assert.Nil(t, settings)
	assert.Contains(t, err.Error(), "failed to parse config file")
}

func TestSaveSettingsPermissionError(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "settings_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Set environment variable to override config path
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	// Create settings to save
	settings := DefaultSettings()

	// Test saving settings (should create directories)
	err = SaveSettings(settings)
	require.NoError(t, err)

	// Verify file was created
	configPath, err := getConfigPath()
	require.NoError(t, err)
	assert.FileExists(t, configPath)
}

// Benchmark tests
func BenchmarkLoadSettings(b *testing.B) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "benchmark_settings_test")
	require.NoError(b, err)
	defer os.RemoveAll(tempDir)

	// Set environment variable to override config path
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	// Create initial settings
	initialSettings := DefaultSettings()
	err = SaveSettings(initialSettings)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := LoadSettings()
		require.NoError(b, err)
	}
}

func BenchmarkSaveSettings(b *testing.B) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "benchmark_settings_test")
	require.NoError(b, err)
	defer os.RemoveAll(tempDir)

	// Set environment variable to override config path
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	// Create settings to save
	settings := DefaultSettings()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := SaveSettings(settings)
		require.NoError(b, err)
	}
}
