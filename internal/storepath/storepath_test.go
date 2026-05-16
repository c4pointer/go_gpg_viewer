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
