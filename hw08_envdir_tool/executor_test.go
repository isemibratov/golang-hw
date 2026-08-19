package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

const helperProcessMarker = "--go-envdir-helper-process"

type helperProcessOutput struct {
	Arguments    []string `json:"arguments"`
	Input        string   `json:"input"`
	Inherited    string   `json:"inherited"`
	Replaced     string   `json:"replaced"`
	Equals       string   `json:"equals"`
	Empty        string   `json:"empty"`
	EmptyIsSet   bool     `json:"emptyIsSet"`
	RemovedIsSet bool     `json:"removedIsSet"`
}

func TestRunCmd(t *testing.T) {
	t.Setenv("ENVDIR_INHERITED", "from parent")
	t.Setenv("ENVDIR_REPLACED", "old value")
	t.Setenv("ENVDIR_REMOVED", "remove me")

	env := Environment{
		"ENVDIR_EMPTY":    {Value: ""},
		"ENVDIR_EQUALS":   {Value: "left=right"},
		"ENVDIR_REPLACED": {Value: "new value"},
		"ENVDIR_REMOVED":  {NeedRemove: true},
	}
	command := helperCommand("23", "argument with spaces", "arg=two")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	gotCode := runCmd(command, env, strings.NewReader("standard input"), &stdout, &stderr)
	if gotCode != 23 {
		t.Errorf("runCmd() code = %d, want 23", gotCode)
	}

	var got helperProcessOutput
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode child stdout %q: %v", stdout.String(), err)
	}
	want := helperProcessOutput{
		Arguments:    []string{"argument with spaces", "arg=two"},
		Input:        "standard input",
		Inherited:    "from parent",
		Replaced:     "new value",
		Equals:       "left=right",
		Empty:        "",
		EmptyIsSet:   true,
		RemovedIsSet: false,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("child output = %#v, want %#v", got, want)
	}
	if gotStderr := stderr.String(); gotStderr != "helper stderr" {
		t.Errorf("child stderr = %q, want %q", gotStderr, "helper stderr")
	}
	if gotParent := os.Getenv("ENVDIR_REPLACED"); gotParent != "old value" {
		t.Errorf("parent ENVDIR_REPLACED = %q, want %q", gotParent, "old value")
	}
	if gotParent := os.Getenv("ENVDIR_REMOVED"); gotParent != "remove me" {
		t.Errorf("parent ENVDIR_REMOVED = %q, want %q", gotParent, "remove me")
	}
}

func TestRunCmdSuccess(t *testing.T) {
	got := runCmd(helperCommand("0"), nil, strings.NewReader(""), io.Discard, io.Discard)
	if got != 0 {
		t.Errorf("runCmd() code = %d, want 0", got)
	}
}

func TestRunCmdUsesOverriddenPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links to executables require extra privileges on Windows")
	}

	t.Setenv("PATH", t.TempDir())
	dir := t.TempDir()
	executable := "go-envdir-path-helper"
	target, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatalf("get test executable path: %v", err)
	}
	if err := os.Symlink(target, filepath.Join(dir, executable)); err != nil {
		t.Fatalf("create test executable symlink: %v", err)
	}
	env := Environment{"PATH": {Value: dir}}

	got := runCmd(helperCommandFor(executable, "23"), env, strings.NewReader(""), io.Discard, io.Discard)
	if got != 23 {
		t.Errorf("runCmd() code = %d, want 23", got)
	}
}

func TestRunCmdStartErrors(t *testing.T) {
	notExecutable := filepath.Join(t.TempDir(), "not-executable")
	writeTestFile(t, notExecutable, []byte("content"))

	tests := []struct {
		name     string
		command  []string
		wantText string
	}{
		{name: "empty command", wantText: "command is required"},
		{
			name:     "missing executable",
			command:  []string{filepath.Join(t.TempDir(), "missing")},
			wantText: "missing",
		},
		{
			name:     "non-executable file",
			command:  []string{notExecutable},
			wantText: "not-executable",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stderr bytes.Buffer
			got := runCmd(test.command, nil, strings.NewReader(""), io.Discard, &stderr)
			if got != errorExitCode {
				t.Errorf("runCmd() code = %d, want %d", got, errorExitCode)
			}
			if !strings.Contains(stderr.String(), test.wantText) {
				t.Errorf("runCmd() stderr = %q, want it to contain %q", stderr.String(), test.wantText)
			}
		})
	}
}

func TestRunCmdSignalExitCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix signal exit codes are not available on Windows")
	}

	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skipf("shell is unavailable: %v", err)
	}
	got := runCmd([]string{shell, "-c", "kill -TERM $$"}, nil, strings.NewReader(""), io.Discard, io.Discard)
	want := 128 + int(syscall.SIGTERM)
	if got != want {
		t.Errorf("runCmd() code = %d, want %d", got, want)
	}
}

func TestRunCmdHelperProcess(t *testing.T) {
	marker := -1
	for index, argument := range os.Args {
		if argument == helperProcessMarker {
			marker = index
			break
		}
	}
	if marker == -1 {
		return
	}
	if len(os.Args) <= marker+1 {
		t.Fatal("helper process arguments do not contain an exit code")
	}

	exitCode, err := strconv.Atoi(os.Args[marker+1])
	if err != nil {
		t.Fatalf("parse helper exit code: %v", err)
	}
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		t.Fatalf("read helper stdin: %v", err)
	}
	empty, emptyIsSet := os.LookupEnv("ENVDIR_EMPTY")
	_, removedIsSet := os.LookupEnv("ENVDIR_REMOVED")
	output := helperProcessOutput{
		Arguments:    os.Args[marker+2:],
		Input:        string(input),
		Inherited:    os.Getenv("ENVDIR_INHERITED"),
		Replaced:     os.Getenv("ENVDIR_REPLACED"),
		Equals:       os.Getenv("ENVDIR_EQUALS"),
		Empty:        empty,
		EmptyIsSet:   emptyIsSet,
		RemovedIsSet: removedIsSet,
	}
	if err := json.NewEncoder(os.Stdout).Encode(output); err != nil {
		t.Fatalf("write helper stdout: %v", err)
	}
	if _, err := fmt.Fprint(os.Stderr, "helper stderr"); err != nil {
		t.Fatalf("write helper stderr: %v", err)
	}

	os.Exit(exitCode)
}

func helperCommand(exitCode string, arguments ...string) []string {
	return helperCommandFor(os.Args[0], exitCode, arguments...)
}

func helperCommandFor(executable, exitCode string, arguments ...string) []string {
	command := []string{
		executable,
		"-test.run=^TestRunCmdHelperProcess$",
		"--",
		helperProcessMarker,
		exitCode,
	}

	return append(command, arguments...)
}
