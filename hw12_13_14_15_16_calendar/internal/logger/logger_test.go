package logger

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestLoggerFiltersMessagesByLevel(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		included []string
		excluded []string
	}{
		{
			name:     "error",
			level:    "error",
			included: []string{"ERROR error message"},
			excluded: []string{"WARN warn message", "INFO info message", "DEBUG debug message"},
		},
		{
			name:     "warn",
			level:    "warn",
			included: []string{"ERROR error message", "WARN  warn message"},
			excluded: []string{"INFO info message", "DEBUG debug message"},
		},
		{
			name:     "info",
			level:    "info",
			included: []string{"ERROR error message", "WARN  warn message", "INFO  info message"},
			excluded: []string{"DEBUG debug message"},
		},
		{
			name:     "debug",
			level:    "debug",
			included: []string{"ERROR error message", "WARN  warn message", "INFO  info message", "DEBUG debug message"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			log, err := newWithWriter(tt.level, &output)
			if err != nil {
				t.Fatalf("create logger: %v", err)
			}

			log.Debug("debug message")
			log.Info("info message")
			log.Warn("warn message")
			log.Error("error message")

			for _, expected := range tt.included {
				if !strings.Contains(output.String(), expected) {
					t.Errorf("expected log output to contain %q, got %q", expected, output.String())
				}
			}
			for _, unexpected := range tt.excluded {
				if strings.Contains(output.String(), unexpected) {
					t.Errorf("expected log output not to contain %q, got %q", unexpected, output.String())
				}
			}
		})
	}
}

func TestNewWithWriterRejectsInvalidArguments(t *testing.T) {
	tests := []struct {
		name   string
		level  string
		writer io.Writer
	}{
		{name: "empty level", writer: &bytes.Buffer{}},
		{name: "unknown level", level: "trace", writer: &bytes.Buffer{}},
		{name: "uppercase level", level: "INFO", writer: &bytes.Buffer{}},
		{name: "nil writer", level: "info"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := newWithWriter(tt.level, tt.writer); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}
