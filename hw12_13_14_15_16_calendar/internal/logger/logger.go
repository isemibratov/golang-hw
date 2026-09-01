// Package logger provides level-based logging to a configurable output stream.
package logger

import (
	"fmt"
	"io"
	"log"
	"os"
)

type level uint8

const (
	errorLevel level = iota
	warnLevel
	infoLevel
	debugLevel
)

const (
	levelNameError = "error"
	levelNameWarn  = "warn"
	levelNameInfo  = "info"
	levelNameDebug = "debug"
)

// Logger writes messages whose severity is enabled by the configured level.
type Logger struct {
	level  level
	logger *log.Logger
}

// New creates a logger that writes to standard output.
func New(level string) (*Logger, error) {
	return newWithWriter(level, os.Stdout)
}

func newWithWriter(logLevel string, writer io.Writer) (*Logger, error) {
	if writer == nil {
		return nil, fmt.Errorf("logger writer is nil")
	}

	parsedLevel, err := parseLevel(logLevel)
	if err != nil {
		return nil, err
	}

	return &Logger{
		level:  parsedLevel,
		logger: log.New(writer, "", log.Ldate|log.Ltime|log.Lmicroseconds|log.LUTC),
	}, nil
}

// Debug writes a debug-level message when debug logging is enabled.
func (l *Logger) Debug(msg string) {
	l.log(debugLevel, "DEBUG", msg)
}

// Warn writes a warning-level message when warning logging is enabled.
func (l *Logger) Warn(msg string) {
	l.log(warnLevel, "WARN", msg)
}

// Info writes an informational message when informational logging is enabled.
func (l *Logger) Info(msg string) {
	l.log(infoLevel, "INFO", msg)
}

// Error writes an error-level message.
func (l *Logger) Error(msg string) {
	l.log(errorLevel, "ERROR", msg)
}

func (l *Logger) log(messageLevel level, label, msg string) {
	if messageLevel > l.level {
		return
	}

	l.logger.Printf("%-5s %s", label, msg)
}

func parseLevel(value string) (level, error) {
	switch value {
	case levelNameError:
		return errorLevel, nil
	case levelNameWarn:
		return warnLevel, nil
	case levelNameInfo:
		return infoLevel, nil
	case levelNameDebug:
		return debugLevel, nil
	default:
		return 0, fmt.Errorf("unsupported logger level %q", value)
	}
}
