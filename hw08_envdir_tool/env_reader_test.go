package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReadDir(t *testing.T) {
	want := Environment{
		"BAR":   {Value: "bar"},
		"EMPTY": {Value: ""},
		"FOO":   {Value: "   foo\nwith new line"},
		"HELLO": {Value: `"hello"`},
		"UNSET": {NeedRemove: true},
	}

	got, err := ReadDir("testdata/env")
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ReadDir() = %#v, want %#v", got, want)
	}
}

func TestReadDirTransformsValues(t *testing.T) {
	dir := t.TempDir()
	longValue := strings.Repeat("x", 70*1024)
	files := map[string][]byte{
		"CR":               []byte("value\r\nignored"),
		"EMPTY_FIRST_LINE": []byte("\nignored"),
		"IGNORED=NAME":     []byte("ignored"),
		"LONG":             []byte(longValue + " \t\nignored"),
		"NULLS":            []byte("one\x00two\x00three \t"),
		"SPACES":           []byte("  keep leading, trim trailing \t \t\nignored"),
	}
	for name, content := range files {
		writeTestFile(t, filepath.Join(dir, name), content)
	}
	if err := os.Mkdir(filepath.Join(dir, "SUBDIRECTORY"), 0o755); err != nil {
		t.Fatalf("create subdirectory: %v", err)
	}

	want := Environment{
		"CR":               {Value: "value\r"},
		"EMPTY_FIRST_LINE": {Value: ""},
		"LONG":             {Value: longValue},
		"NULLS":            {Value: "one\ntwo\nthree"},
		"SPACES":           {Value: "  keep leading, trim trailing"},
	}

	got, err := ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ReadDir() = %#v, want %#v", got, want)
	}
}

func TestReadDirEmptyDirectory(t *testing.T) {
	got, err := ReadDir(t.TempDir())
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if got == nil {
		t.Fatal("ReadDir() returned a nil environment")
	}
	if len(got) != 0 {
		t.Errorf("ReadDir() = %#v, want an empty environment", got)
	}
}

func TestReadDirErrors(t *testing.T) {
	t.Run("missing directory", func(t *testing.T) {
		_, err := ReadDir(filepath.Join(t.TempDir(), "missing"))
		if err == nil {
			t.Fatal("ReadDir() error = nil, want an error")
		}
	})

	t.Run("path is a file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "file")
		writeTestFile(t, path, []byte("value"))

		_, err := ReadDir(path)
		if err == nil {
			t.Fatal("ReadDir() error = nil, want an error")
		}
	})

	t.Run("environment file cannot be read", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Symlink(filepath.Join(dir, "missing"), filepath.Join(dir, "BROKEN")); err != nil {
			t.Skipf("cannot create a broken symlink: %v", err)
		}

		_, err := ReadDir(dir)
		if err == nil {
			t.Fatal("ReadDir() error = nil, want an error")
		}
	})
}

func writeTestFile(t *testing.T, path string, content []byte) {
	t.Helper()

	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write test file %q: %v", path, err)
	}
}
