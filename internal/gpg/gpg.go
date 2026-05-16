// Package gpg wraps the system "gpg" binary with a small, typed surface that
// the rest of the application can call. It intentionally hides the exec.Cmd
// plumbing so concerns like passphrase handling, stdout/stderr separation
// and execution timeouts can be tightened in one place.
package gpg

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// defaultTimeout caps the time any single gpg invocation may run. It guards
// the UI from hanging forever on a broken pinentry, a stuck NFS mount, or
// gpg sitting on stdin waiting for input we never send.
const defaultTimeout = 30 * time.Second

// Decrypt runs `gpg --decrypt` on filePath. If passphrase is non-empty it is
// fed to gpg over stdin (never the argv) using --pinentry-mode loopback +
// --passphrase-fd 0, so it is not visible to other users via
// /proc/<pid>/cmdline. An empty passphrase relies on the running gpg-agent.
//
// Returns gpg's stdout (the plaintext) and stderr (status / warnings)
// separately so callers can surface error messages without ever exposing
// plaintext to the UI or logs.
func Decrypt(filePath, passphrase string) (plaintext, stderr []byte, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	cmd := buildDecryptCmd(ctx, filePath, passphrase)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	err = cmd.Run()
	return stdoutBuf.Bytes(), stderrBuf.Bytes(), err
}

// buildDecryptCmd assembles the gpg decrypt command. Exposed (unexported)
// for testing to assert that the passphrase never lands in argv.
func buildDecryptCmd(ctx context.Context, filePath, passphrase string) *exec.Cmd {
	if passphrase == "" {
		return exec.CommandContext(ctx, "gpg", "--batch", "--decrypt", filePath)
	}
	cmd := exec.CommandContext(ctx, "gpg", "--batch",
		"--pinentry-mode", "loopback",
		"--passphrase-fd", "0",
		"--decrypt", filePath)
	cmd.Stdin = strings.NewReader(passphrase + "\n")
	return cmd
}

// EncryptBytes encrypts the given plaintext to outputFile for the given GPG
// recipient. The plaintext is streamed to gpg through stdin so it never
// touches disk in cleartext (no temp file). gpg writes the ciphertext
// directly to outputFile, so only stderr is returned for diagnostics.
func EncryptBytes(plaintext []byte, outputFile, recipient string) (stderr []byte, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gpg", "--batch", "--yes", "--recipient", recipient,
		"--output", outputFile, "--encrypt")
	cmd.Stdin = bytes.NewReader(plaintext)
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	err = cmd.Run()
	return stderrBuf.Bytes(), err
}

// ListRecipientKeyID inspects an encrypted GPG file and returns the first
// recipient key ID it can extract (16 hex chars). It also returns gpg's
// stderr so callers can surface errors without leaking packet metadata
// from stdout.
func ListRecipientKeyID(filePath string) (keyID string, stderr []byte, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gpg", "--batch", "--list-packets", filePath)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	if err = cmd.Run(); err != nil {
		return "", stderrBuf.Bytes(), fmt.Errorf("gpg --list-packets failed: %w", err)
	}
	return parseRecipientKeyID(stdoutBuf.String()), stderrBuf.Bytes(), nil
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
