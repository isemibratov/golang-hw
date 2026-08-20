package hw09structvalidator

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

var (
	// ErrNotStruct indicates that Validate received a value other than a struct.
	ErrNotStruct = errors.New("validation input is not a struct")
	// ErrInvalidTag indicates malformed validator syntax or arguments.
	ErrInvalidTag = errors.New("invalid validation tag")
	// ErrUnsupportedType indicates a tagged field whose type cannot be validated.
	ErrUnsupportedType = errors.New("unsupported field type")
	// ErrUnsupportedValidator indicates an unknown validator name.
	ErrUnsupportedValidator = errors.New("unsupported validator")

	// ErrLen indicates a failed len validation.
	ErrLen = errors.New("length validation failed")
	// ErrRegexp indicates a failed regexp validation.
	ErrRegexp = errors.New("regular expression validation failed")
	// ErrIn indicates a failed in validation.
	ErrIn = errors.New("set membership validation failed")
	// ErrMin indicates a failed min validation.
	ErrMin = errors.New("minimum validation failed")
	// ErrMax indicates a failed max validation.
	ErrMax = errors.New("maximum validation failed")
)

// ValidationError describes a validation failure for one struct field.
type ValidationError struct {
	Field string
	Err   error
}

// ValidationErrors contains every validation failure found in a struct.
type ValidationErrors []ValidationError

// Error implements the error interface.
func (validationErrors ValidationErrors) Error() string {
	messages := make([]string, len(validationErrors))
	for index, validationError := range validationErrors {
		messages[index] = fmt.Sprintf("%s: %v", validationError.Field, validationError.Err)
	}

	return strings.Join(messages, "; ")
}

// Is reports whether any contained validation failure matches target.
func (validationErrors ValidationErrors) Is(target error) bool {
	for _, validationError := range validationErrors {
		if errors.Is(validationError.Err, target) {
			return true
		}
	}

	return false
}

type validationRule func(value reflect.Value) error

// Validate validates exported fields of a struct according to their validate tags.
func Validate(v interface{}) error {
	value := reflect.ValueOf(v)
	if !value.IsValid() || value.Kind() != reflect.Struct {
		return ErrNotStruct
	}

	valueType := value.Type()
	validationErrors := make(ValidationErrors, 0)
	for index := 0; index < value.NumField(); index++ {
		structField := valueType.Field(index)
		if !structField.IsExported() {
			continue
		}

		tag, ok := structField.Tag.Lookup("validate")
		if !ok || tag == "" {
			continue
		}

		field := value.Field(index)
		kind, err := validatedKind(field)
		if err != nil {
			return fmt.Errorf("field %q: %w", structField.Name, err)
		}

		rules, err := parseValidationRules(tag, kind)
		if err != nil {
			return fmt.Errorf("field %q: %w", structField.Name, err)
		}

		validationErrors = append(validationErrors, validateField(structField.Name, field, rules)...)
	}

	if len(validationErrors) == 0 {
		return nil
	}

	return validationErrors
}

func validatedKind(field reflect.Value) (reflect.Kind, error) {
	kind := field.Kind()
	if kind == reflect.Slice {
		kind = field.Type().Elem().Kind()
	}
	if kind != reflect.Int && kind != reflect.String {
		return reflect.Invalid, fmt.Errorf("%w: %s", ErrUnsupportedType, field.Type())
	}

	return kind, nil
}

func parseValidationRules(tag string, kind reflect.Kind) ([]validationRule, error) {
	rawRules := strings.Split(tag, "|")
	rules := make([]validationRule, 0, len(rawRules))
	for _, rawRule := range rawRules {
		name, argument, found := strings.Cut(rawRule, ":")
		if !found || name == "" || argument == "" {
			return nil, fmt.Errorf("%w: malformed rule %q", ErrInvalidTag, rawRule)
		}

		rule, err := parseValidationRule(name, argument, kind)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}

	return rules, nil
}

func parseValidationRule(name, argument string, kind reflect.Kind) (validationRule, error) {
	switch name {
	case "len":
		if kind == reflect.String {
			return newStringLengthRule(argument)
		}
	case "regexp":
		if kind == reflect.String {
			return newStringRegexpRule(argument)
		}
	case "in":
		if kind == reflect.String {
			return newStringInRule(argument), nil
		}
		if kind == reflect.Int {
			return newIntInRule(argument)
		}
	case "min":
		if kind == reflect.Int {
			return newIntMinRule(argument)
		}
	case "max":
		if kind == reflect.Int {
			return newIntMaxRule(argument)
		}
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedValidator, name)
	}

	return nil, fmt.Errorf("%w: validator %q cannot be used with %s", ErrInvalidTag, name, kind)
}

func newStringLengthRule(argument string) (validationRule, error) {
	want, err := strconv.Atoi(argument)
	if err != nil || want < 0 {
		return nil, fmt.Errorf("%w: len argument %q is not a non-negative integer", ErrInvalidTag, argument)
	}

	return func(value reflect.Value) error {
		actual := utf8.RuneCountInString(value.String())
		if actual == want {
			return nil
		}

		return fmt.Errorf("%w: got %d characters, want %d", ErrLen, actual, want)
	}, nil
}

func newStringRegexpRule(argument string) (validationRule, error) {
	expression, err := regexp.Compile(`\A(` + argument + `)\z`)
	if err != nil {
		return nil, fmt.Errorf("%w: cannot compile regexp %q: %v", ErrInvalidTag, argument, err)
	}

	return func(value reflect.Value) error {
		if expression.MatchString(value.String()) {
			return nil
		}

		return fmt.Errorf("%w: value does not match %q", ErrRegexp, argument)
	}, nil
}

func newStringInRule(argument string) validationRule {
	allowed := make(map[string]struct{})
	for _, item := range strings.Split(argument, ",") {
		allowed[item] = struct{}{}
	}

	return func(value reflect.Value) error {
		if _, ok := allowed[value.String()]; ok {
			return nil
		}

		return fmt.Errorf("%w: %q is not in %q", ErrIn, value.String(), argument)
	}
}

func newIntMinRule(argument string) (validationRule, error) {
	minimum, err := parseIntArgument("min", argument)
	if err != nil {
		return nil, err
	}

	return func(value reflect.Value) error {
		if value.Int() >= minimum {
			return nil
		}

		return fmt.Errorf("%w: got %d, minimum is %d", ErrMin, value.Int(), minimum)
	}, nil
}

func newIntMaxRule(argument string) (validationRule, error) {
	maximum, err := parseIntArgument("max", argument)
	if err != nil {
		return nil, err
	}

	return func(value reflect.Value) error {
		if value.Int() <= maximum {
			return nil
		}

		return fmt.Errorf("%w: got %d, maximum is %d", ErrMax, value.Int(), maximum)
	}, nil
}

func newIntInRule(argument string) (validationRule, error) {
	items := strings.Split(argument, ",")
	allowed := make(map[int64]struct{}, len(items))
	for _, item := range items {
		number, err := parseIntArgument("in", item)
		if err != nil {
			return nil, err
		}
		allowed[number] = struct{}{}
	}

	return func(value reflect.Value) error {
		if _, ok := allowed[value.Int()]; ok {
			return nil
		}

		return fmt.Errorf("%w: %d is not in %q", ErrIn, value.Int(), argument)
	}, nil
}

func parseIntArgument(validator, argument string) (int64, error) {
	value, err := strconv.ParseInt(argument, 10, strconv.IntSize)
	if err != nil {
		return 0, fmt.Errorf("%w: %s argument %q is not an integer", ErrInvalidTag, validator, argument)
	}

	return value, nil
}

func validateField(fieldName string, field reflect.Value, rules []validationRule) ValidationErrors {
	if field.Kind() != reflect.Slice {
		return validateValue(fieldName, field, -1, rules)
	}

	validationErrors := make(ValidationErrors, 0)
	for index := 0; index < field.Len(); index++ {
		validationErrors = append(validationErrors, validateValue(fieldName, field.Index(index), index, rules)...)
	}

	return validationErrors
}

func validateValue(fieldName string, value reflect.Value, index int, rules []validationRule) ValidationErrors {
	validationErrors := make(ValidationErrors, 0)
	for _, rule := range rules {
		err := rule(value)
		if err == nil {
			continue
		}
		if index >= 0 {
			err = fmt.Errorf("element %d: %w", index, err)
		}
		validationErrors = append(validationErrors, ValidationError{Field: fieldName, Err: err})
	}

	return validationErrors
}
