package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCopy(t *testing.T) {
	const sourceData = "0123456789"

	tests := []struct {
		name     string
		input    string
		offset   int64
		limit    int64
		expected string
	}{
		{name: "whole file", input: sourceData, expected: sourceData},
		{name: "limited prefix", input: sourceData, limit: 4, expected: "0123"},
		{name: "offset without limit", input: sourceData, offset: 3, expected: "3456789"},
		{name: "offset and limit", input: sourceData, offset: 3, limit: 4, expected: "3456"},
		{name: "limit exceeds remainder", input: sourceData, offset: 7, limit: 100, expected: "789"},
		{name: "offset at EOF", input: sourceData, offset: 10, limit: 100},
		{name: "empty source"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			temporaryDirectory := t.TempDir()
			source := filepath.Join(temporaryDirectory, "input.txt")
			destination := filepath.Join(temporaryDirectory, "output.txt")
			writeFile(t, source, []byte(test.input))
			writeFile(t, destination, []byte("stale destination data"))

			if err := Copy(source, destination, test.offset, test.limit); err != nil {
				t.Fatalf("Copy() returned an error: %v", err)
			}
			if actual := string(readFile(t, destination)); actual != test.expected {
				t.Fatalf("destination contents = %q, want %q", actual, test.expected)
			}
		})
	}
}

func TestCopyRejectsInvalidArgumentsWithoutChangingDestination(t *testing.T) {
	temporaryDirectory := t.TempDir()
	source := filepath.Join(temporaryDirectory, "input.txt")
	writeFile(t, source, []byte("data"))

	tests := []struct {
		name          string
		offset        int64
		limit         int64
		expectedError error
	}{
		{name: "negative offset", offset: -1},
		{name: "negative limit", limit: -1},
		{name: "offset exceeds file size", offset: 5, expectedError: ErrOffsetExceedsFileSize},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			destination := filepath.Join(t.TempDir(), "output.txt")
			writeFile(t, destination, []byte("keep me"))

			err := Copy(source, destination, test.offset, test.limit)
			if err == nil {
				t.Fatal("Copy() returned nil")
			}
			if test.expectedError != nil && !errors.Is(err, test.expectedError) {
				t.Fatalf("Copy() error = %v, want %v", err, test.expectedError)
			}
			if actual := string(readFile(t, destination)); actual != "keep me" {
				t.Fatalf("destination changed after validation error: %q", actual)
			}
		})
	}
}

func TestCopyRejectsUnsupportedSource(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "output.txt")

	if err := Copy(t.TempDir(), destination, 0, 0); !errors.Is(err, ErrUnsupportedFile) {
		t.Fatalf("Copy() error = %v, want %v", err, ErrUnsupportedFile)
	}
}

func TestCopyReturnsFilesystemError(t *testing.T) {
	temporaryDirectory := t.TempDir()
	err := Copy(
		filepath.Join(temporaryDirectory, "missing.txt"),
		filepath.Join(temporaryDirectory, "output.txt"),
		0,
		0,
	)

	if err == nil {
		t.Fatal("Copy() returned nil for a missing source")
	}
}

func TestCopyRejectsSameFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")
	writeFile(t, path, []byte("important contents"))

	if err := Copy(path, path, 0, 0); err == nil {
		t.Fatal("Copy() returned nil for identical source and destination")
	}
	if actual := string(readFile(t, path)); actual != "important contents" {
		t.Fatalf("source changed while copying onto itself: %q", actual)
	}
}

func assertCompletedProgress(t *testing.T, progress string) {
	t.Helper()

	if strings.Count(progress, "%") < 2 {
		t.Fatalf("progress does not contain multiple updates: %q", progress)
	}
	if !strings.Contains(progress, "100%") {
		t.Fatalf("progress does not reach 100%%: %q", progress)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %q: %v", path, err)
	}
	return data
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()

	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}
