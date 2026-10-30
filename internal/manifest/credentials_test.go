package manifest

import (
	"errors"
	"slices"
	"testing"
)

func TestPlaceholders(t *testing.T) {
	tests := []struct {
		in   string
		want []string
		err  bool
	}{
		{in: "plain", want: nil},
		{in: "Bearer ${GITHUB_TOKEN}", want: []string{"GITHUB_TOKEN"}},
		{in: "${_A}:${b1}", want: []string{"_A", "b1"}},
		{in: "$HOME and $", want: nil},
		{in: "${1A}", err: true},
		{in: "${A-B}", err: true},
		{in: "${}", err: true},
		{in: "${OPEN", err: true},
	}
	for _, tt := range tests {
		got, err := Placeholders(tt.in)
		if tt.err != errors.Is(err, ErrBadPlaceholder) {
			t.Errorf("%q: got error %v, want error %v", tt.in, err, tt.err)
		}
		if !tt.err && !slices.Equal(got, tt.want) {
			t.Errorf("%q: got %q, want %q", tt.in, got, tt.want)
		}
	}
}
