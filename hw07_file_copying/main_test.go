package main

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestRun(t *testing.T) {
	temporaryDirectory := t.TempDir()
	source := filepath.Join(temporaryDirectory, "input.txt")
	destination := filepath.Join(temporaryDirectory, "output.txt")
	writeFile(t, source, []byte("0123456789"))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run(
		[]string{"-from", source, "-to", destination, "-offset", "2", "-limit", "4"},
		&stdout,
		&stderr,
	)

	if exitCode != exitSuccess {
		t.Fatalf("run() exit code = %d, want %d; stderr: %s", exitCode, exitSuccess, stderr.String())
	}
	if actual := string(readFile(t, destination)); actual != "2345" {
		t.Fatalf("destination contents = %q, want %q", actual, "2345")
	}
	assertCompletedProgress(t, stdout.String())
	if stderr.Len() != 0 {
		t.Fatalf("run() wrote to stderr on success: %q", stderr.String())
	}
}

func TestRunRejectsInvalidArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "missing source", args: []string{"-to", "output.txt"}},
		{name: "missing destination", args: []string{"-from", "input.txt"}},
		{name: "invalid offset", args: []string{"-offset", "not-a-number"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			exitCode := run(test.args, &stdout, &stderr)

			if exitCode == exitSuccess {
				t.Fatal("run() returned a successful exit code")
			}
			if stderr.Len() == 0 {
				t.Fatal("run() did not explain the error")
			}
		})
	}
}

func TestRunReportsCopyError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	temporaryDirectory := t.TempDir()
	exitCode := run(
		[]string{
			"-from", filepath.Join(temporaryDirectory, "missing.txt"),
			"-to", filepath.Join(temporaryDirectory, "output.txt"),
		},
		&stdout,
		&stderr,
	)

	if exitCode == exitSuccess {
		t.Fatal("run() returned a successful exit code")
	}
	if stderr.Len() == 0 {
		t.Fatal("run() did not explain the copy error")
	}
}
