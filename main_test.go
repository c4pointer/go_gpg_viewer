package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go_gpg_viewer/scanpassstore"
	"go_gpg_viewer/settings"
)

func TestSplitPasswordAndMetadata(t *testing.T) {
	cases := []struct {
		name      string
		plaintext string
		wantPw    string
		wantMeta  string
	}{
		{"password only", "hunter2", "hunter2", ""},
		{"password + metadata", "hunter2\nuser: alice", "hunter2", "user: alice"},
		{"multi-line metadata", "hunter2\nuser: alice\nnotes: x", "hunter2", "user: alice\nnotes: x"},
		{"empty input", "", "", ""},
		{"only newline", "\n", "", ""},
		{"trailing newline", "hunter2\n", "hunter2", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pw, meta := splitPasswordAndMetadata(c.plaintext)
			assert.Equal(t, c.wantPw, pw)
			assert.Equal(t, c.wantMeta, meta)
		})
	}
}

func TestJoinPasswordAndMetadataIsInverseOfSplit(t *testing.T) {
	originals := []string{
		"hunter2",
		"hunter2\nuser: alice",
		"hunter2\nuser: alice\nnotes: x",
	}
	for _, orig := range originals {
		t.Run(orig, func(t *testing.T) {
			pw, meta := splitPasswordAndMetadata(orig)
			assert.Equal(t, orig, joinPasswordAndMetadata(pw, meta))
		})
	}
}

func TestJoinPasswordAndMetadataEmptyMeta(t *testing.T) {
	assert.Equal(t, "hunter2", joinPasswordAndMetadata("hunter2", ""))
}

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
