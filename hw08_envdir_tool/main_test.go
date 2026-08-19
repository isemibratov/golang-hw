package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunErrors(t *testing.T) {
	tests := []struct {
		name      string
		arguments []string
		wantText  string
	}{
		{
			name:      "missing arguments",
			arguments: []string{"go-envdir"},
			wantText:  "usage:",
		},
		{
			name:      "cannot read environment directory",
			arguments: []string{"go-envdir", filepath.Join(t.TempDir(), "missing"), "command"},
			wantText:  "read environment",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stderr bytes.Buffer

			got := run(test.arguments, &stderr)
			if got != errorExitCode {
				t.Errorf("run() code = %d, want %d", got, errorExitCode)
			}
			if !strings.Contains(stderr.String(), test.wantText) {
				t.Errorf("run() stderr = %q, want it to contain %q", stderr.String(), test.wantText)
			}
		})
	}
}
