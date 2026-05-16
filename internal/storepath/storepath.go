// Package storepath holds path-resolution helpers for the password store.
// It is kept separate from main so the validation logic can be unit-tested
// without dragging in the Fyne UI dependency tree.
package storepath

import (
	"fmt"
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
