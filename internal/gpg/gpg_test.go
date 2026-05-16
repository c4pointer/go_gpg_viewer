package gpg

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildDecryptCmdKeepsPassphraseOutOfArgv(t *testing.T) {
	const secret = "hunter2-super-secret"
	cmd := buildDecryptCmd("/tmp/file.gpg", secret)

	for _, arg := range cmd.Args {
		assert.NotContains(t, arg, secret,
			"passphrase must never appear in argv (visible via /proc/<pid>/cmdline)")
	}
	assert.Contains(t, cmd.Args, "--passphrase-fd")
	assert.Contains(t, cmd.Args, "loopback")

	require.NotNil(t, cmd.Stdin, "passphrase must be piped via stdin")
	piped, err := io.ReadAll(cmd.Stdin)
	require.NoError(t, err)
	assert.Equal(t, secret+"\n", string(piped))
}

func TestBuildDecryptCmdNoPassphraseHasNoStdin(t *testing.T) {
	cmd := buildDecryptCmd("/tmp/file.gpg", "")
	assert.Nil(t, cmd.Stdin)
	assert.NotContains(t, strings.Join(cmd.Args, " "), "--passphrase-fd")
	assert.NotContains(t, strings.Join(cmd.Args, " "), "loopback")
}

func TestParseRecipientKeyID(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want string
	}{
		{
			name: "single packet with keyid",
			out: `:pubkey enc packet: version 3, algo 1, keyid 1234567890ABCDEF
:encrypted data packet:`,
			want: "1234567890ABCDEF",
		},
		{
			name: "multiple recipients picks the first one",
			out: `:pubkey enc packet: version 3, algo 1, keyid AAAAAAAAAAAAAAAA
:pubkey enc packet: version 3, algo 1, keyid BBBBBBBBBBBBBBBB
:encrypted data packet:`,
			want: "AAAAAAAAAAAAAAAA",
		},
		{
			name: "no keyid line",
			out:  ":symmetric key encrypted packet:\n:encrypted data packet:\n",
			want: "",
		},
		{
			name: "keyid too short is skipped",
			out:  "fake keyid 0123\n",
			want: "",
		},
		{
			name: "empty input",
			out:  "",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, parseRecipientKeyID(tt.out))
		})
	}
}
