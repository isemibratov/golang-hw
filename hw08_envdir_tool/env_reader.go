package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Environment map[string]EnvValue

// EnvValue helps to distinguish between empty files and files with the first empty line.
type EnvValue struct {
	Value      string
	NeedRemove bool
}

// ReadDir reads a specified directory and returns map of env variables.
// Variables represented as files where filename is name of variable, file first line is a value.
func ReadDir(dir string) (Environment, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read environment directory %q: %w", dir, err)
	}

	environment := make(Environment, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || strings.ContainsRune(name, '=') {
			continue
		}

		path := filepath.Join(dir, name)
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read environment variable %q: %w", name, err)
		}

		value := EnvValue{NeedRemove: len(content) == 0}
		if !value.NeedRemove {
			if lineEnd := bytes.IndexByte(content, '\n'); lineEnd >= 0 {
				content = content[:lineEnd]
			}
			content = bytes.ReplaceAll(content, []byte{0}, []byte{'\n'})
			value.Value = strings.TrimRight(string(content), " \t")
		}
		environment[name] = value
	}

	return environment, nil
}
