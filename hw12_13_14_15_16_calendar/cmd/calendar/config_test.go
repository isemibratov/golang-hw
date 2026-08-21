package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	path := writeConfig(t, `
[logger]
level = "debug"

[http]
host = "::1"
port = 9090

[storage]
type = "sql"
dsn = "postgres://calendar@localhost/calendar"
`)

	config, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if config.Logger.Level != "debug" {
		t.Errorf("unexpected logger level: %q", config.Logger.Level)
	}
	if config.HTTP.Address() != "[::1]:9090" {
		t.Errorf("unexpected HTTP address: %q", config.HTTP.Address())
	}
	if config.Storage.Type != "sql" {
		t.Errorf("unexpected storage type: %q", config.Storage.Type)
	}
	if config.Storage.DSN != "postgres://calendar@localhost/calendar" {
		t.Errorf("unexpected storage DSN: %q", config.Storage.DSN)
	}
}

func TestLoadConfigKeepsDefaults(t *testing.T) {
	path := writeConfig(t, "[logger]\nlevel = \"warn\"\n")

	config, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if config.Logger.Level != "warn" {
		t.Errorf("unexpected logger level: %q", config.Logger.Level)
	}
	if config.HTTP.Address() != "0.0.0.0:8080" {
		t.Errorf("unexpected default HTTP address: %q", config.HTTP.Address())
	}
	if config.Storage.Type != "memory" {
		t.Errorf("unexpected default storage type: %q", config.Storage.Type)
	}
}

func TestLoadConfigRejectsInvalidFile(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		if _, err := LoadConfig(filepath.Join(t.TempDir(), "missing.toml")); err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("malformed TOML", func(t *testing.T) {
		path := writeConfig(t, "[logger\nlevel = \"info\"\n")
		if _, err := LoadConfig(path); err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("unknown field", func(t *testing.T) {
		path := writeConfig(t, "[logger]\nlevel = \"info\"\nunknown = true\n")
		if _, err := LoadConfig(path); err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("empty path", func(t *testing.T) {
		if _, err := LoadConfig(" "); err == nil {
			t.Fatal("expected an error")
		}
	})
}

func TestConfigValidate(t *testing.T) {
	valid := NewConfig()

	tests := []struct {
		name   string
		change func(*Config)
	}{
		{
			name: "empty logger level",
			change: func(config *Config) {
				config.Logger.Level = ""
			},
		},
		{
			name: "uppercase logger level",
			change: func(config *Config) {
				config.Logger.Level = "INFO"
			},
		},
		{
			name: "unknown logger level",
			change: func(config *Config) {
				config.Logger.Level = "trace"
			},
		},
		{
			name: "empty HTTP host",
			change: func(config *Config) {
				config.HTTP.Host = " "
			},
		},
		{
			name: "zero HTTP port",
			change: func(config *Config) {
				config.HTTP.Port = 0
			},
		},
		{
			name: "HTTP port above maximum",
			change: func(config *Config) {
				config.HTTP.Port = 65536
			},
		},
		{
			name: "unknown storage type",
			change: func(config *Config) {
				config.Storage.Type = "filesystem"
			},
		},
		{
			name: "SQL storage without DSN",
			change: func(config *Config) {
				config.Storage.Type = "sql"
				config.Storage.DSN = " "
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := valid
			tt.change(&config)

			if err := config.Validate(); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
