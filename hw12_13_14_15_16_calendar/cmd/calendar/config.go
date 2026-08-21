// Package main assembles and runs the calendar service.
package main

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	storageTypeMemory = "memory"
	storageTypeSQL    = "sql"
	loggerLevelError  = "error"
	loggerLevelWarn   = "warn"
	loggerLevelInfo   = "info"
	loggerLevelDebug  = "debug"
)

// Config contains settings for all calendar components.
type Config struct {
	Logger  LoggerConf  `toml:"logger"`
	HTTP    HTTPConf    `toml:"http"`
	Storage StorageConf `toml:"storage"`
}

// LoggerConf contains logging settings.
type LoggerConf struct {
	Level string `toml:"level"`
}

// HTTPConf contains HTTP server settings.
type HTTPConf struct {
	Host string `toml:"host"`
	Port int    `toml:"port"`
}

// Address returns the HTTP listen address.
func (c HTTPConf) Address() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

// StorageConf contains storage selection and connection settings.
type StorageConf struct {
	Type string `toml:"type"`
	DSN  string `toml:"dsn"`
}

// NewConfig returns a configuration populated with default values.
func NewConfig() Config {
	return Config{
		Logger: LoggerConf{
			Level: loggerLevelInfo,
		},
		HTTP: HTTPConf{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Storage: StorageConf{
			Type: storageTypeMemory,
		},
	}
}

// LoadConfig reads, strictly decodes, and validates a TOML configuration file.
func LoadConfig(path string) (Config, error) {
	if strings.TrimSpace(path) == "" {
		return Config{}, fmt.Errorf("config path is empty")
	}

	config := NewConfig()
	metadata, err := toml.DecodeFile(path, &config)
	if err != nil {
		return Config{}, fmt.Errorf("decode config %q: %w", path, err)
	}

	if undecoded := metadata.Undecoded(); len(undecoded) != 0 {
		return Config{}, fmt.Errorf("config %q contains unknown fields: %v", path, undecoded)
	}

	if err := config.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate config %q: %w", path, err)
	}

	return config, nil
}

// Validate checks that all configuration values are supported.
func (c Config) Validate() error {
	switch c.Logger.Level {
	case loggerLevelError, loggerLevelWarn, loggerLevelInfo, loggerLevelDebug:
	default:
		return fmt.Errorf("unsupported logger level %q", c.Logger.Level)
	}

	if strings.TrimSpace(c.HTTP.Host) == "" {
		return fmt.Errorf("http host is empty")
	}
	if c.HTTP.Port < 1 || c.HTTP.Port > 65535 {
		return fmt.Errorf("http port must be between 1 and 65535, got %d", c.HTTP.Port)
	}

	switch c.Storage.Type {
	case storageTypeMemory:
	case storageTypeSQL:
		if strings.TrimSpace(c.Storage.DSN) == "" {
			return fmt.Errorf("storage DSN is required for sql storage")
		}
	default:
		return fmt.Errorf("unsupported storage type %q", c.Storage.Type)
	}

	return nil
}
