// Package main assembles and runs the calendar service.
package main

import internalconfig "github.com/isemibratov/golang-hw/hw12_13_14_15_16_calendar/internal/config"

const (
	storageTypeMemory = internalconfig.StorageTypeMemory
	storageTypeSQL    = internalconfig.StorageTypeSQL
)

type (
	// Config contains settings for the calendar API service.
	Config = internalconfig.Calendar
	// LoggerConf contains logging settings.
	LoggerConf = internalconfig.Logger
	// HTTPConf contains HTTP server settings.
	HTTPConf = internalconfig.HTTP
	// StorageConf contains storage selection and connection settings.
	StorageConf = internalconfig.Storage
)

// NewConfig returns a configuration populated with default values.
func NewConfig() Config {
	return internalconfig.NewCalendar()
}

// LoadConfig reads, strictly decodes, and validates a TOML configuration file.
func LoadConfig(path string) (Config, error) {
	return internalconfig.LoadCalendar(path)
}
