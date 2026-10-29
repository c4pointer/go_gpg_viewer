// Package password generates cryptographically random passwords for new
// password-store entries.
package password

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// Default character sets. The "safe" symbol subset deliberately excludes
// characters that tend to cause trouble in shell paste / URL encoding
// (quotes, backticks, backslash, whitespace).
const (
	Lowercase = "abcdefghijklmnopqrstuvwxyz"
	Uppercase = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	Digits    = "0123456789"
	Symbols   = "!@#$%^&*()-_=+[]{};:,.<>?/"
)

// DefaultCharset is the alphabet used when the caller does not pass one
// explicitly. It mixes both letter cases, digits and a curated symbol set
// so the result satisfies the typical "complex password" requirements.
const DefaultCharset = Lowercase + Uppercase + Digits + Symbols

// Generate returns a random password of the given length drawn from
// charset. It uses crypto/rand for selection (rejection-free, no modulo
// bias) and returns an error only if the entropy source itself fails.
func Generate(length int, charset string) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("password length must be positive, got %d", length)
	}
	if len(charset) == 0 {
		return "", fmt.Errorf("charset must not be empty")
	}
	runes := []rune(charset)
	max := big.NewInt(int64(len(runes)))

	out := make([]rune, length)
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("crypto/rand failed: %w", err)
		}
		out[i] = runes[n.Int64()]
	}
	return string(out), nil
}
