//go:build !bench
// +build !bench

package hw10programoptimization

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetDomainStat_JSONCompatibility(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		data    string
		want    DomainStat
		wantErr bool
	}{
		{
			name: "case insensitive field name",
			data: `{"email":"reader@example.com"}`,
			want: DomainStat{testEmailDomain: 1},
		},
		{
			name: "duplicate null preserves the string",
			data: `{"Email":"reader@example.com","Email":null}`,
			want: DomainStat{testEmailDomain: 1},
		},
		{
			name: "null integer is allowed",
			data: `{"Id":42,"Id":null,"Email":"reader@example.com"}`,
			want: DomainStat{testEmailDomain: 1},
		},
		{
			name: "large number in an unknown field",
			data: `{"Metadata":[0.1e100],"Email":"reader@example.com"}`,
			want: DomainStat{testEmailDomain: 1},
		},
		{
			name: "escaped prefix is not Email",
			data: `{"\u0045":"reader@example.com"}`,
			want: DomainStat{},
		},
		{
			name: "null suffix is not Email",
			data: `{"Email\u0000":"reader@example.com"}`,
			want: DomainStat{},
		},
		{
			name: "blank lines",
			data: "\n \t\r\n" + `{"Email":"reader@example.com"}` + "\n\n",
			want: DomainStat{testEmailDomain: 1},
		},
		{
			name:    "integer overflow",
			data:    `{"Id":9223372036854775808,"Email":"reader@example.com"}`,
			wantErr: true,
		},
		{
			name:    "integer overflow beyond uint64",
			data:    `{"Id":25000000000000000000,"Email":"reader@example.com"}`,
			wantErr: true,
		},
		{
			name:    "quoted integer is rejected",
			data:    `{"Id":"42","Email":"reader@example.com"}`,
			wantErr: true,
		},
		{
			name:    "fractional integer is rejected",
			data:    `{"Id":1.5,"Email":"reader@example.com"}`,
			wantErr: true,
		},
		{
			name:    "invalid JSON in an unknown field",
			data:    `{"Metadata":[1,],"Email":"reader@example.com"}`,
			wantErr: true,
		},
		{
			name:    "unescaped tab in a string",
			data:    "{\"Name\":\"raw\ttab\",\"Email\":\"reader@example.com\"}",
			wantErr: true,
		},
		{
			name:    "comma between top level objects",
			data:    `{}, {"Email":"reader@example.com"}`,
			wantErr: true,
		},
		{
			name:    "colon before a record",
			data:    `: {"Email":"reader@example.com"}`,
			wantErr: true,
		},
		{
			name:    "invalid records cannot be joined",
			data:    "{\"Email\":\n{\"Email\":\"reader@example.com\"}\n\"reader@example.com\"}",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stat, err := GetDomainStat(strings.NewReader(tt.data), testTopLevelDomain)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.want, stat)
		})
	}
}

func TestGetDomainStat_TruncatedRecordWithReadError(t *testing.T) {
	t.Parallel()

	readErr := errors.New("reader failed during a record")
	r := &finalDataErrorReader{data: `{"Email":`, err: readErr}
	stat, err := GetDomainStat(r, testTopLevelDomain)
	require.ErrorIs(t, err, readErr)
	require.Nil(t, stat)
}

func TestGetDomainStat_LargeInputWithInvalidField(t *testing.T) {
	t.Parallel()

	const valid = "{\"Email\":\"reader@example.com\"}\n"
	const invalid = "{\"Name\":42}\n"
	data := strings.Repeat(valid, 20_000)
	for name, input := range map[string]string{
		"before valid records": invalid + data,
		"after valid records":  data + invalid,
	} {
		t.Run(name, func(t *testing.T) {
			stat, err := GetDomainStat(strings.NewReader(input), testTopLevelDomain)
			require.Error(t, err)
			require.Nil(t, stat)
		})
	}
}
