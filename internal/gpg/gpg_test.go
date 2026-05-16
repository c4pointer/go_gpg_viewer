package gpg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
