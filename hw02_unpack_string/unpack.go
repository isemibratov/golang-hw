package hw02unpackstring

import (
	"errors"
	"strconv"
	"strings"
	"unicode"
)

var ErrInvalidString = errors.New("invalid string")

func Unpack(input string) (string, error) {
	runes := []rune(input)
	var result strings.Builder

	for i := 0; i < len(runes); i++ {
		current := runes[i]
		if unicode.IsDigit(current) {
			return "", ErrInvalidString
		}

		if current == '\\' {
			i++
			if i == len(runes) || (runes[i] != '\\' && !unicode.IsDigit(runes[i])) {
				return "", ErrInvalidString
			}
			current = runes[i]
		}

		repeatCount := 1
		if i+1 < len(runes) && unicode.IsDigit(runes[i+1]) {
			var err error
			repeatCount, err = strconv.Atoi(string(runes[i+1]))
			if err != nil {
				return "", ErrInvalidString
			}
			i++
		}

		result.WriteString(strings.Repeat(string(current), repeatCount))
	}

	return result.String(), nil
}
