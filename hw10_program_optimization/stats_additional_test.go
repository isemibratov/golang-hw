//go:build !bench
// +build !bench

package hw10programoptimization

import (
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/require"
)

const (
	testTopLevelDomain = "com"
	testEmailDomain    = "example.com"
)

func TestGetDomainStat_StreamBoundaries(t *testing.T) {
	t.Parallel()

	const record = `{"Email":"reader@Example.com"}`
	tests := []struct {
		name string
		data string
		want DomainStat
	}{
		{
			name: "empty stream",
			want: DomainStat{},
		},
		{
			name: "single record without newline",
			data: record,
			want: DomainStat{testEmailDomain: 1},
		},
		{
			name: "trailing newline",
			data: record + "\n" + record + "\n",
			want: DomainStat{testEmailDomain: 2},
		},
		{
			name: "CRLF separators",
			data: record + "\r\n" + record + "\r\n",
			want: DomainStat{testEmailDomain: 2},
		},
		{
			name: "last record without newline",
			data: record + "\n" + record,
			want: DomainStat{testEmailDomain: 2},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stat, err := GetDomainStat(strings.NewReader(tt.data), testTopLevelDomain)
			require.NoError(t, err)
			require.Equal(t, tt.want, stat)
		})
	}
}

func TestGetDomainStat_JSONEscapesAndUnrelatedFields(t *testing.T) {
	t.Parallel()

	data := `{"Name":"escaped \"Email\":\"decoy@Wrong.com\"","Email":"al\u0069ce\u0040Exa\u006Dple\u002ecom"}
{"Metadata":{"Email":"another@Wrong.com"},"Address":"{\"Email\":\"fake@Wrong.com\"}","Email":"bob@Example.com"}
{"\u0045mail":"carol@Other.com"}`

	stat, err := GetDomainStat(strings.NewReader(data), testTopLevelDomain)
	require.NoError(t, err)
	require.Equal(t, DomainStat{testEmailDomain: 2, "other.com": 1}, stat)
}

func TestGetDomainStat_ReaderBoundaries(t *testing.T) {
	t.Parallel()

	const data = "{\"Email\":\"first@Example.com\"}\n{\"Email\":\"second@Example.com\"}"
	tests := []struct {
		name string
		wrap func(io.Reader) io.Reader
	}{
		{name: "one byte per read", wrap: iotest.OneByteReader},
		{name: "partial reads", wrap: iotest.HalfReader},
		{name: "final data together with EOF", wrap: iotest.DataErrReader},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stat, err := GetDomainStat(tt.wrap(strings.NewReader(data)), testTopLevelDomain)
			require.NoError(t, err)
			require.Equal(t, DomainStat{testEmailDomain: 2}, stat)
		})
	}
}

func TestGetDomainStat_ResetsUserBetweenRecords(t *testing.T) {
	t.Parallel()

	data := `{"Email":"first@Example.com"}
{}
{"Email":"second@Other.com"}
{"Email":""}
{"Email":"third@Example.com"}
{"Email":null}`

	stat, err := GetDomainStat(strings.NewReader(data), testTopLevelDomain)
	require.NoError(t, err)
	require.Equal(t, DomainStat{testEmailDomain: 2, "other.com": 1}, stat)
}

func TestGetDomainStat_InvalidInput(t *testing.T) {
	t.Parallel()

	const validRecord = "{\"Email\":\"valid@Example.com\"}\n"
	tests := []struct {
		name string
		data string
	}{
		{name: "truncated JSON", data: `{"Email":"broken@Example.com"`},
		{name: "invalid JSON escape", data: `{"Email":"broken\x40Example.com"}`},
		{name: "trailing garbage", data: `{"Email":"valid@Example.com"} invalid`},
		{name: "number instead of Email", data: `{"Email":42}`},
		{name: "string instead of Id", data: `{"Id":"one"}`},
		{name: "boolean instead of Name", data: `{"Name":true}`},
		{name: "number instead of Username", data: `{"Username":42}`},
		{name: "object instead of Phone", data: `{"Phone":{}}`},
		{name: "array instead of Password", data: `{"Password":[]}`},
		{name: "number instead of Address", data: `{"Address":42}`},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stat, err := GetDomainStat(strings.NewReader(validRecord+tt.data), testTopLevelDomain)
			require.Error(t, err)
			require.Nil(t, stat)
		})
	}
}

func TestGetDomainStat_ReaderError(t *testing.T) {
	t.Parallel()

	readErr := errors.New("reader failed")
	const record = "{\"Email\":\"before-error@Example.com\"}\n"
	tests := []struct {
		name   string
		reader io.Reader
	}{
		{
			name:   "before any data",
			reader: iotest.ErrReader(readErr),
		},
		{
			name:   "after a valid record",
			reader: io.MultiReader(strings.NewReader(record), iotest.ErrReader(readErr)),
		},
		{
			name: "data and error in the same read",
			reader: iotest.DataErrReader(
				io.MultiReader(strings.NewReader(record), iotest.ErrReader(readErr)),
			),
		},
		{
			name:   "one shot error with final data",
			reader: &finalDataErrorReader{data: record, err: readErr},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stat, err := GetDomainStat(tt.reader, testTopLevelDomain)
			require.ErrorIs(t, err, readErr)
			require.Nil(t, stat)
		})
	}
}

type finalDataErrorReader struct {
	data string
	err  error
}

func (r *finalDataErrorReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}

	n := copy(p, r.data)
	r.data = r.data[n:]
	if len(r.data) == 0 {
		return n, r.err
	}
	return n, nil
}

func TestGetDomainStat_LongRecord(t *testing.T) {
	t.Parallel()

	data := `{"Name":"` + strings.Repeat("x", 70*1024) + `","Email":"long@Example.com"}` +
		"\n" + `{"Email":"short@Example.com"}`

	stat, err := GetDomainStat(strings.NewReader(data), testTopLevelDomain)
	require.NoError(t, err)
	require.Equal(t, DomainStat{testEmailDomain: 2}, stat)
}

func TestGetDomainStat_MoreThanOneHundredThousandRecords(t *testing.T) {
	const recordCount = 100_001
	data := strings.Repeat("{\"Email\":\"many@Example.com\"}\n", recordCount)

	stat, err := GetDomainStat(strings.NewReader(data), testTopLevelDomain)
	require.NoError(t, err)
	require.Equal(t, DomainStat{testEmailDomain: recordCount}, stat)
}

func TestGetDomainStat_RegexpMatching(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		email  string
		domain string
		want   DomainStat
	}{
		{
			name:   "match inside a domain label",
			email:  "person@Example.company",
			domain: testTopLevelDomain,
			want:   DomainStat{"example.company": 1},
		},
		{
			name:   "match before the top level domain",
			email:  "person@Example.com.org",
			domain: testTopLevelDomain,
			want:   DomainStat{"example.com.org": 1},
		},
		{
			name:   "match in the local part",
			email:  "first.com@Example.org",
			domain: testTopLevelDomain,
			want:   DomainStat{"example.org": 1},
		},
		{
			name:   "regexp metacharacter in domain argument",
			email:  "person@Example.com",
			domain: "c.m",
			want:   DomainStat{testEmailDomain: 1},
		},
		{
			name:   "matching is case sensitive",
			email:  "person@Example.COM",
			domain: testTopLevelDomain,
			want:   DomainStat{},
		},
		{
			name:   "matching email without separator is skipped",
			email:  testEmailDomain,
			domain: testTopLevelDomain,
			want:   DomainStat{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stat, err := GetDomainStat(strings.NewReader(`{"Email":"`+tt.email+`"}`), tt.domain)
			require.NoError(t, err)
			require.Equal(t, tt.want, stat)
		})
	}
}

func TestGetDomainStat_InvalidRegexp(t *testing.T) {
	t.Parallel()

	stat, err := GetDomainStat(strings.NewReader(`{}`), "[")
	require.Error(t, err)
	require.Nil(t, stat)
}
