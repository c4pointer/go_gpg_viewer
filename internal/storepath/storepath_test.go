package storepath

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveRecord(t *testing.T) {
	target := filepath.Join(t.TempDir(), "store")
	require.NoError(t, os.MkdirAll(target, 0700))
	absTarget, err := filepath.Abs(target)
	require.NoError(t, err)

	t.Run("plain name", func(t *testing.T) {
		got, err := ResolveRecord(target, "gmail")
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(absTarget, "gmail.gpg"), got)
	})

	t.Run("nested name", func(t *testing.T) {
		got, err := ResolveRecord(target, filepath.Join("Finance", "bank"))
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(absTarget, "Finance", "bank.gpg"), got)
	})

	t.Run("rejects path traversal", func(t *testing.T) {
		_, err := ResolveRecord(target, filepath.Join("..", "..", "etc", "passwd"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "escapes the password store")
	})

	t.Run("rejects absolute path", func(t *testing.T) {
		_, err := ResolveRecord(target, "/etc/passwd")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be relative")
	})

	t.Run("rejects empty name", func(t *testing.T) {
		_, err := ResolveRecord(target, "  ")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")
	})

	t.Run("rejects resolving to a sibling of the store", func(t *testing.T) {
		base := filepath.Base(target)
		_, err := ResolveRecord(target, filepath.Join("..", base+"-other"))
		require.Error(t, err)
	})
}

func writeRecord(t *testing.T, path string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	require.NoError(t, os.WriteFile(path, []byte("ciphertext"), 0600))
}

func TestMoveRecord(t *testing.T) {
	t.Run("into a new nested directory and prunes the old one", func(t *testing.T) {
		root := t.TempDir()
		oldPath := filepath.Join(root, "Old", "Deep", "gmail.gpg")
		newPath := filepath.Join(root, "Mail", "Work", "gmail-work.gpg")
		writeRecord(t, oldPath)

		require.NoError(t, MoveRecord(root, oldPath, newPath))

		data, err := os.ReadFile(newPath)
		require.NoError(t, err)
		assert.Equal(t, "ciphertext", string(data))
		assert.NoFileExists(t, oldPath)
		assert.NoDirExists(t, filepath.Join(root, "Old"))
		assert.DirExists(t, root)
	})

	t.Run("keeps directories that still hold files", func(t *testing.T) {
		root := t.TempDir()
		oldPath := filepath.Join(root, "Finance", "bank.gpg")
		writeRecord(t, oldPath)
		writeRecord(t, filepath.Join(root, "Finance", "card.gpg"))

		require.NoError(t, MoveRecord(root, oldPath, filepath.Join(root, "bank.gpg")))

		assert.FileExists(t, filepath.Join(root, "bank.gpg"))
		assert.FileExists(t, filepath.Join(root, "Finance", "card.gpg"))
	})

	t.Run("refuses to overwrite an existing record", func(t *testing.T) {
		root := t.TempDir()
		oldPath := filepath.Join(root, "a.gpg")
		newPath := filepath.Join(root, "b.gpg")
		writeRecord(t, oldPath)
		writeRecord(t, newPath)

		assert.Error(t, MoveRecord(root, oldPath, newPath))
		assert.FileExists(t, oldPath)
		assert.FileExists(t, newPath)
	})
}

func TestRemoveRecord(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Only", "entry.gpg")
	writeRecord(t, path)

	require.NoError(t, RemoveRecord(root, path))

	assert.NoFileExists(t, path)
	assert.NoDirExists(t, filepath.Join(root, "Only"))
	assert.DirExists(t, root)
}

func TestPruneEmptyDirsNeverLeavesRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "store")
	require.NoError(t, os.MkdirAll(root, 0700))
	outside := filepath.Join(parent, "outside")
	require.NoError(t, os.MkdirAll(outside, 0700))

	pruneEmptyDirs(root, root)
	pruneEmptyDirs(root, outside)

	assert.DirExists(t, root)
	assert.DirExists(t, outside)
}
