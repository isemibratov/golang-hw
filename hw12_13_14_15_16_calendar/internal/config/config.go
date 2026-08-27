// Package config loads and validates configuration for the calendar services.
package config

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

const (
	StorageTypeMemory       = "memory"
	StorageTypeSQL          = "sql"
	maxKafkaTopicNameLength = 249

	loggerLevelError = "error"
	loggerLevelWarn  = "warn"
	loggerLevelInfo  = "info"
	loggerLevelDebug = "debug"
)

// Duration is a TOML string containing a value accepted by time.ParseDuration.
type Duration time.Duration

// NewDuration creates a configuration duration from a time.Duration.
func NewDuration(value time.Duration) Duration {
	return Duration(value)
}

// Value returns the standard-library duration value.
func (d Duration) Value() time.Duration {
	return time.Duration(d)
}

// UnmarshalText parses a duration such as "500ms" or "1m".
func (d *Duration) UnmarshalText(text []byte) error {
	value, err := time.ParseDuration(string(text))
	if err != nil {
		return fmt.Errorf("parse duration %q: %w", text, err)
	}
	*d = Duration(value)
	return nil
}

// Logger contains logging settings shared by all services.
type Logger struct {
	Level string `toml:"level"`
}

// HTTP contains the API listen settings.
type HTTP struct {
	Host string `toml:"host"`
	Port int    `toml:"port"`
}

// Address returns the HTTP listen address.
func (c HTTP) Address() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

// Storage contains storage selection and connection settings.
type Storage struct {
	Type string `toml:"type"`
	DSN  string `toml:"dsn"`
}

// Kafka contains settings shared by the producer and consumer.
type Kafka struct {
	Brokers         []string `toml:"brokers"`
	Topic           string   `toml:"topic"`
	GroupID         string   `toml:"group_id"`
	ConnectTimeout  Duration `toml:"connect_timeout"`
	RetryInitial    Duration `toml:"retry_initial"`
	RetryMax        Duration `toml:"retry_max"`
	WriteTimeout    Duration `toml:"write_timeout"`
	MaxMessageBytes int      `toml:"max_message_bytes"`
}

// SchedulerSettings controls periodic notification and cleanup jobs.
type SchedulerSettings struct {
	Interval       Duration `toml:"interval"`
	BatchSize      int      `toml:"batch_size"`
	RetentionYears int      `toml:"retention_years"`
}

// Calendar contains the API service configuration.
type Calendar struct {
	Logger  Logger  `toml:"logger"`
	HTTP    HTTP    `toml:"http"`
	Storage Storage `toml:"storage"`
}

// Scheduler contains the scheduler service configuration.
type Scheduler struct {
	Logger    Logger            `toml:"logger"`
	Storage   Storage           `toml:"storage"`
	Kafka     Kafka             `toml:"kafka"`
	Scheduler SchedulerSettings `toml:"scheduler"`
}

// Storer contains the storer service configuration.
type Storer struct {
	Logger  Logger  `toml:"logger"`
	Storage Storage `toml:"storage"`
	Kafka   Kafka   `toml:"kafka"`
}

// NewCalendar returns API configuration populated with defaults.
func NewCalendar() Calendar {
	return Calendar{
		Logger:  Logger{Level: loggerLevelInfo},
		HTTP:    HTTP{Host: "0.0.0.0", Port: 8080},
		Storage: Storage{Type: StorageTypeMemory},
	}
}

// NewScheduler returns scheduler configuration populated with defaults.
func NewScheduler() Scheduler {
	return Scheduler{
		Logger:  Logger{Level: loggerLevelInfo},
		Storage: Storage{Type: StorageTypeSQL},
		Kafka:   defaultKafka(),
		Scheduler: SchedulerSettings{
			Interval:       NewDuration(time.Minute),
			BatchSize:      100,
			RetentionYears: 1,
		},
	}
}

// NewStorer returns storer configuration populated with defaults.
func NewStorer() Storer {
	config := Storer{
		Logger:  Logger{Level: loggerLevelInfo},
		Storage: Storage{Type: StorageTypeSQL},
		Kafka:   defaultKafka(),
	}
	config.Kafka.GroupID = "calendar-storer"
	return config
}

func defaultKafka() Kafka {
	return Kafka{
		Brokers:         []string{"localhost:9092"},
		Topic:           "calendar.notifications",
		ConnectTimeout:  NewDuration(5 * time.Second),
		RetryInitial:    NewDuration(500 * time.Millisecond),
		RetryMax:        NewDuration(10 * time.Second),
		WriteTimeout:    NewDuration(10 * time.Second),
		MaxMessageBytes: 1 << 20,
	}
}

// LoadCalendar reads, strictly decodes, and validates an API configuration file.
func LoadCalendar(path string) (Calendar, error) {
	config := NewCalendar()
	if err := decode(path, &config); err != nil {
		return Calendar{}, err
	}
	if err := config.Validate(); err != nil {
		return Calendar{}, validationError(path, err)
	}
	return config, nil
}

// LoadScheduler reads, strictly decodes, and validates a scheduler configuration file.
func LoadScheduler(path string) (Scheduler, error) {
	config := NewScheduler()
	if err := decode(path, &config); err != nil {
		return Scheduler{}, err
	}
	if err := config.Validate(); err != nil {
		return Scheduler{}, validationError(path, err)
	}
	return config, nil
}

// LoadStorer reads, strictly decodes, and validates a storer configuration file.
func LoadStorer(path string) (Storer, error) {
	config := NewStorer()
	if err := decode(path, &config); err != nil {
		return Storer{}, err
	}
	if err := config.Validate(); err != nil {
		return Storer{}, validationError(path, err)
	}
	return config, nil
}

func decode(path string, target interface{}) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("config path is empty")
	}

	metadata, err := toml.DecodeFile(path, target)
	if err != nil {
		return fmt.Errorf("decode config %q: %w", path, err)
	}
	if undecoded := metadata.Undecoded(); len(undecoded) != 0 {
		return fmt.Errorf("config %q contains unknown fields: %v", path, undecoded)
	}
	return nil
}

func validationError(path string, err error) error {
	return fmt.Errorf("validate config %q: %w", path, err)
}

// Validate checks API configuration values.
func (c Calendar) Validate() error {
	if err := c.Logger.validate(); err != nil {
		return err
	}
	if strings.TrimSpace(c.HTTP.Host) == "" {
		return fmt.Errorf("http host is empty")
	}
	if c.HTTP.Port < 1 || c.HTTP.Port > 65535 {
		return fmt.Errorf("http port must be between 1 and 65535, got %d", c.HTTP.Port)
	}
	return c.Storage.validate(true)
}

// Validate checks scheduler configuration values.
func (c Scheduler) Validate() error {
	if err := c.Logger.validate(); err != nil {
		return err
	}
	if err := c.Storage.validate(false); err != nil {
		return err
	}
	if err := c.Kafka.validate(false); err != nil {
		return err
	}
	if c.Scheduler.Interval.Value() <= 0 {
		return fmt.Errorf("scheduler interval must be positive")
	}
	if c.Scheduler.BatchSize <= 0 {
		return fmt.Errorf("scheduler batch size must be positive")
	}
	if c.Scheduler.RetentionYears <= 0 {
		return fmt.Errorf("scheduler retention years must be positive")
	}
	return nil
}

// Validate checks storer configuration values.
func (c Storer) Validate() error {
	if err := c.Logger.validate(); err != nil {
		return err
	}
	if err := c.Storage.validate(false); err != nil {
		return err
	}
	return c.Kafka.validate(true)
}

func (c Logger) validate() error {
	switch c.Level {
	case loggerLevelError, loggerLevelWarn, loggerLevelInfo, loggerLevelDebug:
		return nil
	default:
		return fmt.Errorf("unsupported logger level %q", c.Level)
	}
}

func (c Storage) validate(allowMemory bool) error {
	switch c.Type {
	case StorageTypeMemory:
		if allowMemory {
			return nil
		}
		return fmt.Errorf("storage type %q is not supported by this service", c.Type)
	case StorageTypeSQL:
		if strings.TrimSpace(c.DSN) == "" {
			return fmt.Errorf("storage DSN is required for sql storage")
		}
		return nil
	default:
		return fmt.Errorf("unsupported storage type %q", c.Type)
	}
}

func (c Kafka) validate(requireGroup bool) error {
	if len(c.Brokers) == 0 {
		return fmt.Errorf("at least one Kafka broker is required")
	}
	for _, broker := range c.Brokers {
		if strings.TrimSpace(broker) == "" {
			return fmt.Errorf("Kafka broker address is empty")
		}
	}
	if err := validateKafkaTopicName(c.Topic); err != nil {
		return err
	}
	if requireGroup && strings.TrimSpace(c.GroupID) == "" {
		return fmt.Errorf("Kafka consumer group ID is empty")
	}
	if c.ConnectTimeout.Value() <= 0 {
		return fmt.Errorf("Kafka connect timeout must be positive")
	}
	if c.RetryInitial.Value() <= 0 || c.RetryMax.Value() <= 0 {
		return fmt.Errorf("Kafka retry delays must be positive")
	}
	if c.RetryInitial.Value() > c.RetryMax.Value() {
		return fmt.Errorf("Kafka initial retry delay exceeds maximum retry delay")
	}
	if c.WriteTimeout.Value() <= 0 {
		return fmt.Errorf("Kafka write timeout must be positive")
	}
	if c.MaxMessageBytes <= 0 {
		return fmt.Errorf("Kafka maximum message size must be positive")
	}
	return nil
}

func validateKafkaTopicName(topic string) error {
	switch {
	case strings.TrimSpace(topic) == "":
		return fmt.Errorf("Kafka topic is empty")
	case len(topic) > maxKafkaTopicNameLength:
		return fmt.Errorf("Kafka topic exceeds %d bytes", maxKafkaTopicNameLength)
	case topic == "." || topic == "..":
		return fmt.Errorf("Kafka topic %q is reserved", topic)
	}

	for index := 0; index < len(topic); index++ {
		if !isKafkaTopicNameByte(topic[index]) {
			return fmt.Errorf("Kafka topic contains invalid character %q", topic[index])
		}
	}
	return nil
}

func isKafkaTopicNameByte(value byte) bool {
	return value >= 'a' && value <= 'z' ||
		value >= 'A' && value <= 'Z' ||
		value >= '0' && value <= '9' ||
		value == '.' || value == '_' || value == '-'
}
