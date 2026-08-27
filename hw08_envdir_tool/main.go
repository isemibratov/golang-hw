package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	os.Exit(run(os.Args, os.Stderr))
}

func run(arguments []string, stderr io.Writer) int {
	if len(arguments) < 3 {
		_, _ = fmt.Fprintln(stderr, "usage: go-envdir <directory> <command> [arguments...]")

		return errorExitCode
	}

	environment, err := ReadDir(arguments[1])
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "go-envdir: read environment: %v\n", err)

		return errorExitCode
	}

	return RunCmd(arguments[2:], environment)
}
