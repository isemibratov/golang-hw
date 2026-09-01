package hw10programoptimization

import (
	"bufio"
	"bytes"
	stdjson "encoding/json"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/segmentio/encoding/json"
)

type User struct {
	ID       userID
	Name     string
	Username string
	Email    string
	Phone    string
	Password string
	Address  string
}

type DomainStat map[string]int

// userID preserves int overflow checks in both JSON decoders.
type userID int

func (id *userID) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		return nil
	}
	value, err := strconv.ParseInt(string(data), 10, strconv.IntSize)
	if err != nil {
		return err
	}
	*id = userID(value)
	return nil
}

// GetDomainStat counts email domains in a stream of JSON user records.
// Matching uses the original regular expression: `\.` + domain.
func GetDomainStat(r io.Reader, domain string) (DomainStat, error) {
	matcher, err := regexp.Compile(`\.` + domain)
	if err != nil {
		return nil, fmt.Errorf("compile domain regexp: %w", err)
	}

	result := make(DomainStat)
	scanner := bufio.NewScanner(r)
	scanner.Buffer(nil, math.MaxInt)
	var user User
	for scanner.Scan() {
		line := bytes.Trim(scanner.Bytes(), " \t\r")
		if len(line) == 0 {
			continue
		}
		// Missing or null fields must not retain values from the previous record.
		user = User{}
		decode := json.Unmarshal
		if bytes.IndexByte(line, '\\') >= 0 {
			// Preserve stdlib handling of escaped keys, including embedded NULs.
			decode = stdjson.Unmarshal
		}
		if err := decode(line, &user); err != nil {
			if readErr := scanner.Err(); readErr != nil {
				err = readErr
			}
			return nil, fmt.Errorf("get users error: %w", err)
		}
		if !matcher.MatchString(user.Email) {
			continue
		}
		_, emailDomain, ok := strings.Cut(user.Email, "@")
		if !ok {
			continue
		}
		result[strings.ToLower(emailDomain)]++
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("get users error: %w", err)
	}
	return result, nil
}
