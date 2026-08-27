package hw02unpackstring

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnpack(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "repeat runes", input: "a4bc2d5e", expected: "aaaabccddddde"},
		{name: "string without counts", input: "abccd", expected: "abccd"},
		{name: "empty string", input: "", expected: ""},
		{name: "zero count", input: "aaa0b", expected: "aab"},
		{name: "unicode runes", input: "я2界3🙂2", expected: "яя界界界🙂🙂"},
		{name: "newline rune", input: "d\n5abc", expected: "d\n\n\n\n\nabc"},
		{name: "escaped digits", input: `qwe\4\5`, expected: "qwe45"},
		{name: "escaped digit with count", input: `qwe\45`, expected: "qwe44444"},
		{name: "escaped slash with count", input: `qwe\\5`, expected: `qwe\\\\\`},
		{name: "escaped slash and digit", input: `qwe\\\3`, expected: `qwe\3`},
		{name: "escaped zero with count", input: `qwe\05`, expected: "qwe00000"},
		{name: "escaped slash with zero count", input: `qwe\\0`, expected: "qwe"},
		{name: "escaped slash", input: `qwe\\`, expected: `qwe\`},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			result, err := Unpack(tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestUnpackInvalidString(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "starts with digit", input: "3abc"},
		{name: "contains digits only", input: "45"},
		{name: "contains multi-digit count", input: "aaa10b"},
		{name: "contains count after zero", input: "a01"},
		{name: "ends with escape", input: `abc\`},
		{name: "escapes ordinary rune", input: `qw\ne`},
		{name: "digit after escaped digit count", input: `qwe\450`},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			result, err := Unpack(tc.input)
			require.Empty(t, result)
			require.Truef(t, errors.Is(err, ErrInvalidString), "actual error %q", err)
		})
	}
}
