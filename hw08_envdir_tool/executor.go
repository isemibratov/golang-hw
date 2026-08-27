package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

const errorExitCode = 111

// RunCmd runs a command + arguments (cmd) with environment variables from env.
func RunCmd(cmd []string, env Environment) int {
	return runCmd(cmd, env, os.Stdin, os.Stdout, os.Stderr)
}

func runCmd(cmd []string, env Environment, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(cmd) == 0 {
		_, _ = fmt.Fprintln(stderr, "go-envdir: command is required")

		return errorExitCode
	}

	environment := mergeEnvironment(env)
	//nolint:gosec // The command is intentionally supplied by the envdir user.
	command := exec.Command(cmd[0], cmd[1:]...)
	command.Env = environment
	if filepath.Base(cmd[0]) == cmd[0] {
		command.Path, command.Err = lookPathInEnvironment(cmd[0], environment)
	}
	command.Stdin = stdin
	command.Stdout = stdout
	command.Stderr = stderr

	err := command.Run()
	if err == nil {
		return 0
	}

	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		exitCode := exitError.ExitCode()
		if exitCode >= 0 {
			return exitCode
		}
		if status, ok := exitError.ProcessState.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return 128 + int(status.Signal())
		}

		return errorExitCode
	}
	_, _ = fmt.Fprintf(stderr, "go-envdir: run %q: %v\n", cmd[0], err)

	return errorExitCode
}

func lookPathInEnvironment(file string, environment []string) (string, error) {
	path, found := lookupEnvironment(environment, "PATH")
	if !found {
		return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
	}

	for _, directory := range filepath.SplitList(path) {
		if directory == "" {
			directory = "."
		}
		candidate := filepath.Join(directory, file)
		if filepath.Base(candidate) == candidate {
			candidate = "." + string(os.PathSeparator) + candidate
		}
		resolved, err := exec.LookPath(candidate)
		if err == nil {
			return resolved, nil
		}
	}

	return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
}

func lookupEnvironment(environment []string, name string) (string, bool) {
	prefix := name + "="
	for index := len(environment) - 1; index >= 0; index-- {
		if strings.HasPrefix(environment[index], prefix) {
			return environment[index][len(prefix):], true
		}
	}

	return "", false
}

func mergeEnvironment(overrides Environment) []string {
	parent := os.Environ()
	result := make([]string, 0, len(parent)+len(overrides))
	for _, variable := range parent {
		name := variable
		if separator := strings.IndexByte(variable, '='); separator >= 0 {
			name = variable[:separator]
		}
		if _, overridden := overrides[name]; !overridden {
			result = append(result, variable)
		}
	}

	for name, value := range overrides {
		if !value.NeedRemove {
			result = append(result, name+"="+value.Value)
		}
	}

	return result
}
