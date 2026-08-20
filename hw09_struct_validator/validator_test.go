package hw09structvalidator

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type UserRole string

type (
	User struct {
		ID     string `json:"id" validate:"len:36"`
		Name   string
		Age    int             `validate:"min:18|max:50"`
		Email  string          `validate:"regexp:^\\w+@\\w+\\.\\w+$"`
		Role   UserRole        `validate:"in:admin,stuff"`
		Phones []string        `validate:"len:11"`
		meta   json.RawMessage //nolint:unused
	}

	App struct {
		Version string `validate:"len:5"`
	}

	Token struct {
		Header    []byte
		Payload   []byte
		Signature []byte
	}

	Response struct {
		Code int    `validate:"in:200,404,500"`
		Body string `json:"omitempty"`
	}
)

func TestValidateValid(t *testing.T) {
	t.Parallel()

	type Score int
	type Alias string
	type Scores []Score
	type Aliases []Alias
	type Profile struct {
		Role    UserRole `validate:"in:admin,stuff"`
		Scores  Scores   `validate:"min:-1|max:10|in:-1,0,10"`
		Aliases Aliases  `validate:"len:3|in:foo,bar"`
		Empty   []string `validate:"len:1"`
	}
	type PrivateFields struct {
		ignored string `validate:"len:100"`
		Skipped bool   `validate:""`
	}
	type Unicode struct {
		Word string `validate:"len:4"`
	}
	type Digits struct {
		Value string `validate:"regexp:\\d+"`
	}

	tests := []struct {
		name  string
		value interface{}
	}{
		{
			name: "user at minimum age",
			value: User{
				ID:     "12345678-1234-1234-1234-123456789012",
				Age:    18,
				Email:  "user@example.com",
				Role:   "admin",
				Phones: []string{"79991234567", "78881234567"},
			},
		},
		{
			name: "user at maximum age",
			value: User{
				ID:     "12345678-1234-1234-1234-123456789012",
				Age:    50,
				Email:  "name@host.test",
				Role:   "stuff",
				Phones: nil,
			},
		},
		{name: "app", value: App{Version: "1.2.3"}},
		{name: "fields without validation tags", value: Token{}},
		{name: "other tags", value: Response{Code: 404}},
		{
			name: "named types and slices",
			value: Profile{
				Role:    "admin",
				Scores:  Scores{-1, 0, 10},
				Aliases: Aliases{"foo", "bar"},
				Empty:   []string{},
			},
		},
		{name: "unexported and empty tags", value: PrivateFields{ignored: "bad"}},
		{name: "unicode length", value: Unicode{Word: "ёжик"}},
		{name: "full regexp match", value: Digits{Value: "123456"}},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if err := Validate(test.value); err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
		})
	}
}

func TestValidateCollectsAllErrors(t *testing.T) {
	t.Parallel()

	type InvalidData struct {
		Exact        string   `validate:"len:4"`
		Digits       string   `validate:"regexp:\\d+"`
		Choice       string   `validate:"in:foo,bar"`
		Minimum      int      `validate:"min:10"`
		Maximum      int      `validate:"max:20"`
		NumberChoice int      `validate:"in:256,1024"`
		Bounds       []int    `validate:"min:0|max:10"`
		Codes        []string `validate:"regexp:^\\d+$|len:2"`
	}

	err := Validate(InvalidData{
		Exact:        "abc",
		Digits:       "12x",
		Choice:       "baz",
		Minimum:      9,
		Maximum:      21,
		NumberChoice: 512,
		Bounds:       []int{-1, 5, 11},
		Codes:        []string{"12", "x", "333"},
	})
	if err == nil {
		t.Fatal("Validate() error = nil, want validation errors")
	}

	var validationErrors ValidationErrors
	if !errors.As(err, &validationErrors) {
		t.Fatalf("Validate() error type = %T, want ValidationErrors", err)
	}
	if got, want := len(validationErrors), 11; got != want {
		t.Fatalf("len(ValidationErrors) = %d, want %d: %v", got, want, err)
	}

	wantErrors := map[ValidationError]int{
		{Field: "Exact", Err: ErrLen}:       1,
		{Field: "Digits", Err: ErrRegexp}:   1,
		{Field: "Choice", Err: ErrIn}:       1,
		{Field: "Minimum", Err: ErrMin}:     1,
		{Field: "Maximum", Err: ErrMax}:     1,
		{Field: "NumberChoice", Err: ErrIn}: 1,
		{Field: "Bounds", Err: ErrMin}:      1,
		{Field: "Bounds", Err: ErrMax}:      1,
		{Field: "Codes", Err: ErrRegexp}:    1,
		{Field: "Codes", Err: ErrLen}:       2,
	}
	for expected, wantCount := range wantErrors {
		gotCount := 0
		for _, validationError := range validationErrors {
			if validationError.Field == expected.Field && errors.Is(validationError.Err, expected.Err) {
				gotCount++
			}
		}
		if gotCount != wantCount {
			t.Errorf(
				"validation error count for field %q and %v = %d, want %d",
				expected.Field,
				expected.Err,
				gotCount,
				wantCount,
			)
		}
	}

	for sentinel, want := range map[error]bool{
		ErrLen: true, ErrRegexp: true, ErrIn: true, ErrMin: true, ErrMax: true, ErrNotStruct: false,
	} {
		if got := errors.Is(err, sentinel); got != want {
			t.Errorf("errors.Is(Validate(), %v) = %v, want %v", sentinel, got, want)
		}
	}

	for field := range map[string]struct{}{"Exact": {}, "Bounds": {}, "Codes": {}} {
		if !strings.Contains(err.Error(), field) {
			t.Errorf("Validate().Error() = %q, want field %q", err, field)
		}
	}
}

func TestValidateRejectsNonStruct(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value interface{}
	}{
		{name: "nil", value: nil},
		{name: "integer", value: 42},
		{name: "slice", value: []int{1}},
		{name: "map", value: map[string]int{}},
		{name: "pointer", value: &User{}},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := Validate(test.value)
			if !errors.Is(err, ErrNotStruct) {
				t.Fatalf("Validate() error = %v, want ErrNotStruct", err)
			}

			var validationErrors ValidationErrors
			if errors.As(err, &validationErrors) {
				t.Fatalf("Validate() error type = %T, want a programming error", err)
			}
		})
	}
}

func TestValidateRejectsInvalidRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value interface{}
		want  error
	}{
		{name: "missing argument", value: struct {
			Value string `validate:"len"`
		}{}, want: ErrInvalidTag},
		{name: "empty rule", value: struct {
			Value string `validate:":1"`
		}{}, want: ErrInvalidTag},
		{name: "empty argument", value: struct {
			Value string `validate:"regexp:"`
		}{}, want: ErrInvalidTag},
		{name: "invalid length", value: struct {
			Value string `validate:"len:nope"`
		}{}, want: ErrInvalidTag},
		{name: "negative length", value: struct {
			Value string `validate:"len:-1"`
		}{}, want: ErrInvalidTag},
		{name: "invalid integer set", value: struct {
			Value int `validate:"in:1,nope"`
		}{}, want: ErrInvalidTag},
		{name: "invalid regexp", value: struct {
			Value string `validate:"regexp:["`
		}{}, want: ErrInvalidTag},
		{name: "trailing separator", value: struct {
			Value string `validate:"len:1|"`
		}{}, want: ErrInvalidTag},
		{name: "unknown validator", value: struct {
			Value string `validate:"required:true"`
		}{}, want: ErrUnsupportedValidator},
		{name: "validator for wrong type", value: struct {
			Value string `validate:"min:1"`
		}{}, want: ErrInvalidTag},
		{name: "unsupported field type", value: struct {
			Value bool `validate:"in:true"`
		}{}, want: ErrUnsupportedType},
		{name: "unsupported slice element", value: struct {
			Value []byte `validate:"min:0"`
		}{}, want: ErrUnsupportedType},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := Validate(test.value)
			if !errors.Is(err, test.want) {
				t.Fatalf("Validate() error = %v, want errors.Is(_, %v)", err, test.want)
			}

			var validationErrors ValidationErrors
			if errors.As(err, &validationErrors) {
				t.Fatalf("Validate() error type = %T, want a programming error", err)
			}
		})
	}
}

func TestValidateReturnsProgrammingErrorInsteadOfPartialResult(t *testing.T) {
	t.Parallel()

	value := struct {
		Invalid string `validate:"len:10"`
		Broken  int    `validate:"max:not-a-number"`
	}{Invalid: "short"}

	err := Validate(value)
	if !errors.Is(err, ErrInvalidTag) {
		t.Fatalf("Validate() error = %v, want ErrInvalidTag", err)
	}

	var validationErrors ValidationErrors
	if errors.As(err, &validationErrors) {
		t.Fatalf("Validate() error type = %T, want a programming error", err)
	}
}
