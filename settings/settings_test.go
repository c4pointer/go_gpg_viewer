package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setTestConfigHome points both HOME and XDG_CONFIG_HOME at dir so that
// LoadSettings/SaveSettings write into the test's temp area on every
// supported platform.
func setTestConfigHome(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
}

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
	tempDir := t.TempDir()
	setTestConfigHome(t, tempDir)

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
	tempDir := t.TempDir()
	setTestConfigHome(t, tempDir)

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
	err = os.MkdirAll(configDir, 0700)
	require.NoError(t, err)

	// Write existing settings
	data, err := json.MarshalIndent(existingSettings, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(configPath, data, 0600)
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
	tempDir := t.TempDir()
	setTestConfigHome(t, tempDir)

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
	err := SaveSettings(settings)
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

// TestSaveSettingsFilePermissions ensures the on-disk config file is
// user-private — it carries the GPG default recipient and password-store
// path, neither of which should be world-readable on shared machines.
func TestSaveSettingsFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file modes not applicable on Windows")
	}
	tempDir := t.TempDir()
	setTestConfigHome(t, tempDir)

	require.NoError(t, SaveSettings(DefaultSettings()))

	configPath, err := getConfigPath()
	require.NoError(t, err)

	info, err := os.Stat(configPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm(),
		"config file must be readable by the owner only")

	dirInfo, err := os.Stat(filepath.Dir(configPath))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0700), dirInfo.Mode().Perm(),
		"config directory must be searchable by the owner only")
}

// TestSaveSettingsAtomic verifies that no half-written settings.tmp file
// is left in the config directory after a successful save.
func TestSaveSettingsAtomic(t *testing.T) {
	tempDir := t.TempDir()
	setTestConfigHome(t, tempDir)

	require.NoError(t, SaveSettings(DefaultSettings()))

	configPath, err := getConfigPath()
	require.NoError(t, err)

	entries, err := os.ReadDir(filepath.Dir(configPath))
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".tmp",
			"atomic write must remove the temp file on success")
	}
}

func TestUpdateSettings(t *testing.T) {
	tempDir := t.TempDir()
	setTestConfigHome(t, tempDir)

	// Create initial settings
	initialSettings := DefaultSettings()
	require.NoError(t, SaveSettings(initialSettings))

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

	require.NoError(t, UpdateSettings(updates))

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
	tempDir := t.TempDir()
	setTestConfigHome(t, tempDir)

	require.NoError(t, SaveSettings(DefaultSettings()))

	// Test updating with invalid key (should be ignored)
	updates := map[string]interface{}{
		"invalid_key": "value",
		"theme":       "dark",
	}

	require.NoError(t, UpdateSettings(updates))

	// Load and verify settings
	updatedSettings, err := LoadSettings()
	require.NoError(t, err)
	assert.NotNil(t, updatedSettings)

	// Verify valid update was applied
	assert.Equal(t, "dark", updatedSettings.Theme)
}

func TestUpdateSettingsInvalidType(t *testing.T) {
	tempDir := t.TempDir()
	setTestConfigHome(t, tempDir)

	require.NoError(t, SaveSettings(DefaultSettings()))

	// Test updating with wrong type (should be ignored)
	updates := map[string]interface{}{
		"window_width": "not_a_number",
		"theme":        "dark",
	}

	require.NoError(t, UpdateSettings(updates))

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
	tempDir := t.TempDir()
	setTestConfigHome(t, tempDir)

	configPath, err := getConfigPath()
	require.NoError(t, err)

	// Ensure config directory exists
	configDir := filepath.Dir(configPath)
	require.NoError(t, os.MkdirAll(configDir, 0700))

	// Write invalid JSON
	require.NoError(t, os.WriteFile(configPath, []byte("invalid json content"), 0600))

	// Test loading corrupted settings
	settings, err := LoadSettings()
	assert.Error(t, err)
	assert.Nil(t, settings)
	assert.Contains(t, err.Error(), "failed to parse config file")
}

// Benchmark tests
func BenchmarkLoadSettings(b *testing.B) {
	tempDir := b.TempDir()
	b.Setenv("HOME", tempDir)
	b.Setenv("XDG_CONFIG_HOME", filepath.Join(tempDir, ".config"))

	require.NoError(b, SaveSettings(DefaultSettings()))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := LoadSettings()
		require.NoError(b, err)
	}
}

func BenchmarkSaveSettings(b *testing.B) {
	tempDir := b.TempDir()
	b.Setenv("HOME", tempDir)
	b.Setenv("XDG_CONFIG_HOME", filepath.Join(tempDir, ".config"))

	settings := DefaultSettings()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		require.NoError(b, SaveSettings(settings))
	}
}
