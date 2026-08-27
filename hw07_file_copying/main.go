package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

const (
	exitSuccess = 0
	exitFailure = 1
	exitUsage   = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("go-cp", flag.ContinueOnError)
	flags.SetOutput(stderr)

	var fromPath, toPath string
	var offset, limit int64
	flags.StringVar(&fromPath, "from", "", "file to read from")
	flags.StringVar(&toPath, "to", "", "file to write to")
	flags.Int64Var(&offset, "offset", 0, "offset in input file")
	flags.Int64Var(&limit, "limit", 0, "limit of bytes to copy")

	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if fromPath == "" {
		_, _ = fmt.Fprintln(stderr, "missing required -from argument")
		return exitUsage
	}
	if toPath == "" {
		_, _ = fmt.Fprintln(stderr, "missing required -to argument")
		return exitUsage
	}

	if err := copyFile(fromPath, toPath, offset, limit, stdout); err != nil {
		_, _ = fmt.Fprintf(stderr, "copy failed: %v\n", err)
		return exitFailure
	}
	return exitSuccess
}
