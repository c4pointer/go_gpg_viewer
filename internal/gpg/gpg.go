// Package gpg wraps the system "gpg" binary with a small, typed surface that
// the rest of the application can call. It intentionally hides the exec.Cmd
// plumbing so concerns like passphrase handling, stdout/stderr separation
// and execution timeouts can be tightened in one place.
package gpg

import (
	"fmt"
	"os/exec"
	"strings"
)

// Decrypt runs `gpg --decrypt` on filePath. If passphrase is non-empty it is
// supplied to gpg; otherwise the running gpg-agent is expected to unlock the
// private key. The returned byte slice is the combined stdout+stderr output
// from gpg (the existing call-sites need it as-is for now).
func Decrypt(filePath, passphrase string) ([]byte, error) {
	var cmd *exec.Cmd
	if passphrase == "" {
		cmd = exec.Command("gpg", "--batch", "--decrypt", filePath)
	} else {
		cmd = exec.Command("gpg", "--batch", "--passphrase", passphrase, "--decrypt", filePath)
	}
	return cmd.CombinedOutput()
}

// EncryptFile encrypts inputFile to outputFile for the given GPG recipient.
// The returned bytes are the combined gpg output for error reporting.
func EncryptFile(inputFile, outputFile, recipient string) ([]byte, error) {
	cmd := exec.Command("gpg", "--batch", "--yes", "--recipient", recipient,
		"--output", outputFile, "--encrypt", inputFile)
	return cmd.CombinedOutput()
}

// ListRecipientKeyID inspects an encrypted GPG file and returns the first
// recipient key ID it can extract (16 hex chars). It also returns the raw
// gpg output so callers can surface errors with context.
func ListRecipientKeyID(filePath string) (keyID string, raw []byte, err error) {
	cmd := exec.Command("gpg", "--batch", "--list-packets", filePath)
	raw, err = cmd.CombinedOutput()
	if err != nil {
		return "", raw, fmt.Errorf("gpg --list-packets failed: %w", err)
	}
	return parseRecipientKeyID(string(raw)), raw, nil
}

// parseRecipientKeyID extracts the first 16-character key ID from gpg
// --list-packets output. Exposed unexported for testing.
func parseRecipientKeyID(output string) string {
	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(line, "keyid") {
			continue
		}
		parts := strings.SplitN(line, "keyid", 2)
		if len(parts) < 2 {
			continue
		}
		keyid := strings.TrimSpace(parts[1])
		if len(keyid) >= 16 {
			return keyid[:16]
		}
	}
	return ""
}
