package password

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateLength(t *testing.T) {
	for _, n := range []int{1, 8, 20, 64, 256} {
		got, err := Generate(n, DefaultCharset)
		require.NoError(t, err)
		assert.Equal(t, n, utf8.RuneCountInString(got), "want %d runes", n)
	}
}

func TestGenerateRejectsInvalidArgs(t *testing.T) {
	_, err := Generate(0, DefaultCharset)
	assert.Error(t, err)

	_, err = Generate(-5, DefaultCharset)
	assert.Error(t, err)

	_, err = Generate(10, "")
	assert.Error(t, err)
}

func TestGenerateUsesOnlyCharsetRunes(t *testing.T) {
	const charset = "abc123"
	for i := 0; i < 50; i++ {
		got, err := Generate(20, charset)
		require.NoError(t, err)
		for _, r := range got {
			assert.True(t, strings.ContainsRune(charset, r),
				"got rune %q not in charset %q", r, charset)
		}
	}
}

// TestGenerateProducesVariety is a weak sanity check that the output is not
// stuck on a single character. With a 64-char alphabet and 20 positions,
// the probability of all positions matching is astronomically small —
// failing this means rand wiring is broken.
func TestGenerateProducesVariety(t *testing.T) {
	got, err := Generate(20, DefaultCharset)
	require.NoError(t, err)

	seen := map[rune]struct{}{}
	for _, r := range got {
		seen[r] = struct{}{}
	}
	assert.Greater(t, len(seen), 4,
		"a 20-char password from a 64-char alphabet should not collapse to %d unique runes", len(seen))
}

func TestGenerateIsDifferentBetweenCalls(t *testing.T) {
	a, err := Generate(32, DefaultCharset)
	require.NoError(t, err)
	b, err := Generate(32, DefaultCharset)
	require.NoError(t, err)
	assert.NotEqual(t, a, b, "two independent 32-char generations must not collide")
}
