package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go_gpg_viewer/scanpassstore"
	"go_gpg_viewer/settings"
)

func TestScanPasswordStoreCLI(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cli_scan_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a .gpg file in the temp dir
	filePath := tempDir + "/test1.gpg"
	err = os.WriteFile(filePath, []byte("dummy"), 0644)
	require.NoError(t, err)

	// Create a settings file in a temp HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	settingsObj := settings.DefaultSettings()
	settingsObj.PasswordStorePath = tempDir
	err = settings.SaveSettings(settingsObj)
	require.NoError(t, err)

	// Load settings and scan
	loaded, err := settings.LoadSettings()
	require.NoError(t, err)
	assert.Equal(t, tempDir, loaded.PasswordStorePath)

	store, err := scanpassstore.ScanPasswordStore(tempDir)
	require.NoError(t, err)
	assert.Contains(t, store.RootFiles, "test1")
}
