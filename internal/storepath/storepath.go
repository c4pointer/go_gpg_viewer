// Package storepath holds path-resolution helpers for the password store.
// It is kept separate from main so the validation logic can be unit-tested
// without dragging in the Fyne UI dependency tree.
package storepath

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ResolveRecord resolves recordName relative to targetPath, enforcing that
// the result stays inside the password store. recordName may include
// forward-slash separated subdirectories, but must not escape via "..", an
// absolute path, or any other trickery. The returned absolute path always
// ends in ".gpg".
func ResolveRecord(targetPath, recordName string) (string, error) {
	if strings.TrimSpace(recordName) == "" {
		return "", fmt.Errorf("record name cannot be empty")
	}
	if filepath.IsAbs(recordName) {
		return "", fmt.Errorf("record name must be relative to the password store")
	}

	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve password store path: %w", err)
	}
	candidate := filepath.Clean(filepath.Join(absTarget, recordName+".gpg"))

	rel, err := filepath.Rel(absTarget, candidate)
	if err != nil {
		return "", fmt.Errorf("failed to validate record path: %w", err)
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("record name %q escapes the password store", recordName)
	}
	return candidate, nil
}

// MoveRecord renames the record file oldPath to newPath inside the store
// rooted at root. Missing directories for newPath are created and
// directories left empty by the move are pruned. An existing record at
// newPath is never overwritten. The ciphertext is moved as-is, so every
// recipient the record was encrypted for is preserved.
func MoveRecord(root, oldPath, newPath string) error {
	if _, err := os.Stat(newPath); err == nil {
		return fmt.Errorf("record %q already exists", newPath)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("failed to check target record: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(newPath), 0700); err != nil {
		return fmt.Errorf("failed to create directory structure: %w", err)
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		return fmt.Errorf("failed to rename record: %w", err)
	}
	pruneEmptyDirs(root, filepath.Dir(oldPath))
	return nil
}

// RemoveRecord deletes the record file at path and prunes directories left
// empty inside the store rooted at root.
func RemoveRecord(root, path string) error {
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to remove record: %w", err)
	}
	pruneEmptyDirs(root, filepath.Dir(path))
	return nil
}

// pruneEmptyDirs removes dir and then its parents for as long as they are
// empty. It stops at root, which is never removed, and never touches
// anything outside of it. Directories holding other files (records, a
// per-directory .gpg-id, ...) are left alone because os.Remove refuses to
// delete a non-empty directory.
func pruneEmptyDirs(root, dir string) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		return
	}
	for {
		rel, err := filepath.Rel(absRoot, dir)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return
		}
		if os.Remove(dir) != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}
