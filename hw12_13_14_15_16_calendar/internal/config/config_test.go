package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadScheduler(t *testing.T) {
	path := writeConfig(t, `
[logger]
level = "debug"

[storage]
type = "sql"
dsn = "postgres://calendar@localhost/calendar"

[kafka]
brokers = ["kafka-a:9092", "kafka-b:9092"]
topic = "calendar.test"
connect_timeout = "3s"
retry_initial = "100ms"
retry_max = "2s"
write_timeout = "4s"
max_message_bytes = 2048

[scheduler]
interval = "15s"
batch_size = 25
retention_years = 1
`)

	config, err := LoadScheduler(path)
	if err != nil {
		t.Fatalf("LoadScheduler() error = %v", err)
	}
	if config.Logger.Level != "debug" || config.Storage.DSN != "postgres://calendar@localhost/calendar" {
		t.Fatalf("unexpected shared configuration: %#v", config)
	}
	if len(config.Kafka.Brokers) != 2 || config.Kafka.Topic != "calendar.test" {
		t.Fatalf("unexpected Kafka configuration: %#v", config.Kafka)
	}
	if config.Kafka.ConnectTimeout.Value() != 3*time.Second ||
		config.Kafka.RetryInitial.Value() != 100*time.Millisecond ||
		config.Kafka.RetryMax.Value() != 2*time.Second {
		t.Fatalf("unexpected Kafka durations: %#v", config.Kafka)
	}
	if config.Scheduler.Interval.Value() != 15*time.Second || config.Scheduler.BatchSize != 25 {
		t.Fatalf("unexpected scheduler configuration: %#v", config.Scheduler)
	}
}

func TestLoadStorerKeepsDefaults(t *testing.T) {
	path := writeConfig(t, `
[storage]
dsn = "postgres://calendar@localhost/calendar"
`)

	config, err := LoadStorer(path)
	if err != nil {
		t.Fatalf("LoadStorer() error = %v", err)
	}
	if config.Kafka.GroupID != "calendar-storer" {
		t.Fatalf("Kafka group ID = %q, want calendar-storer", config.Kafka.GroupID)
	}
	if config.Kafka.Topic != "calendar.notifications" || len(config.Kafka.Brokers) != 1 {
		t.Fatalf("unexpected default Kafka configuration: %#v", config.Kafka)
	}
}

func TestLoadCalendarAppliesEnvironment(t *testing.T) {
	t.Setenv(envLoggerLevel, "debug")
	t.Setenv(envHTTPHost, "127.0.0.1")
	t.Setenv(envHTTPPort, "8888")
	t.Setenv(envStorageType, StorageTypeSQL)
	t.Setenv(envStorageDSN, "postgres://calendar@postgres/calendar")

	config, err := LoadCalendar(writeConfig(t, ""))
	if err != nil {
		t.Fatalf("LoadCalendar() error = %v", err)
	}
	if config.Logger.Level != "debug" {
		t.Fatalf("logger level = %q, want debug", config.Logger.Level)
	}
	if config.HTTP.Host != "127.0.0.1" || config.HTTP.Port != 8888 {
		t.Fatalf("unexpected HTTP configuration: %#v", config.HTTP)
	}
	if config.Storage.Type != StorageTypeSQL ||
		config.Storage.DSN != "postgres://calendar@postgres/calendar" {
		t.Fatalf("unexpected storage configuration: %#v", config.Storage)
	}
}

func TestLoadSchedulerAppliesEnvironment(t *testing.T) {
	t.Setenv(envStorageDSN, "postgres://calendar@postgres/calendar")
	t.Setenv(envKafkaBrokers, "kafka-a:29092, kafka-b:29092")
	t.Setenv(envKafkaTopic, "calendar.integration")
	t.Setenv(envKafkaConnectTimeout, "7s")
	t.Setenv(envKafkaRetryInitial, "250ms")
	t.Setenv(envKafkaRetryMax, "3s")
	t.Setenv(envKafkaWriteTimeout, "8s")
	t.Setenv(envKafkaMaxMessageBytes, "4096")
	t.Setenv(envSchedulerInterval, "500ms")
	t.Setenv(envSchedulerBatchSize, "12")
	t.Setenv(envSchedulerRetentionYears, "2")

	config, err := LoadScheduler(writeConfig(t, ""))
	if err != nil {
		t.Fatalf("LoadScheduler() error = %v", err)
	}
	if len(config.Kafka.Brokers) != 2 || config.Kafka.Brokers[1] != "kafka-b:29092" {
		t.Fatalf("unexpected Kafka brokers: %#v", config.Kafka.Brokers)
	}
	if config.Kafka.Topic != "calendar.integration" || config.Kafka.MaxMessageBytes != 4096 {
		t.Fatalf("unexpected Kafka configuration: %#v", config.Kafka)
	}
	if config.Kafka.ConnectTimeout.Value() != 7*time.Second ||
		config.Kafka.RetryInitial.Value() != 250*time.Millisecond ||
		config.Kafka.RetryMax.Value() != 3*time.Second ||
		config.Kafka.WriteTimeout.Value() != 8*time.Second {
		t.Fatalf("unexpected Kafka durations: %#v", config.Kafka)
	}
	if config.Scheduler.Interval.Value() != 500*time.Millisecond ||
		config.Scheduler.BatchSize != 12 || config.Scheduler.RetentionYears != 2 {
		t.Fatalf("unexpected scheduler configuration: %#v", config.Scheduler)
	}
}

func TestLoadStorerAppliesConsumerGroupEnvironment(t *testing.T) {
	t.Setenv(envStorageDSN, "postgres://calendar@postgres/calendar")
	t.Setenv(envKafkaGroupID, "calendar-integration-storer")

	config, err := LoadStorer(writeConfig(t, ""))
	if err != nil {
		t.Fatalf("LoadStorer() error = %v", err)
	}
	if config.Kafka.GroupID != "calendar-integration-storer" {
		t.Fatalf("Kafka group ID = %q, want calendar-integration-storer", config.Kafka.GroupID)
	}
}

func TestLoadRejectsInvalidEnvironmentValue(t *testing.T) {
	t.Setenv(envStorageDSN, "postgres://calendar@postgres/calendar")
	t.Setenv(envSchedulerInterval, "immediately")

	_, err := LoadScheduler(writeConfig(t, ""))
	if err == nil || !strings.Contains(err.Error(), envSchedulerInterval) {
		t.Fatalf("LoadScheduler() error = %v, want error mentioning %s", err, envSchedulerInterval)
	}
}

func TestLoadRejectsUnknownFieldAndInvalidDuration(t *testing.T) {
	t.Run("unknown field", func(t *testing.T) {
		path := writeConfig(t, "[storage]\ndsn = \"postgres://localhost/calendar\"\nunknown = true\n")
		if _, err := LoadScheduler(path); err == nil {
			t.Fatal("LoadScheduler() error = nil, want an error")
		}
	})

	t.Run("invalid duration", func(t *testing.T) {
		path := writeConfig(t, "[scheduler]\ninterval = \"soon\"\n")
		if _, err := LoadScheduler(path); err == nil {
			t.Fatal("LoadScheduler() error = nil, want an error")
		}
	})
}

func TestSchedulerConfigValidation(t *testing.T) {
	valid := NewScheduler()
	valid.Storage.DSN = "postgres://localhost/calendar"

	tests := []struct {
		name   string
		change func(*Scheduler)
	}{
		{name: "memory storage", change: func(c *Scheduler) { c.Storage.Type = StorageTypeMemory }},
		{name: "empty broker", change: func(c *Scheduler) { c.Kafka.Brokers = []string{" "} }},
		{name: "empty topic", change: func(c *Scheduler) { c.Kafka.Topic = " " }},
		{name: "invalid topic character", change: func(c *Scheduler) { c.Kafka.Topic = "calendar/events" }},
		{name: "reserved topic", change: func(c *Scheduler) { c.Kafka.Topic = ".." }},
		{name: "long topic", change: func(c *Scheduler) {
			c.Kafka.Topic = strings.Repeat("a", maxKafkaTopicNameLength+1)
		}},
		{name: "invalid retry range", change: func(c *Scheduler) {
			c.Kafka.RetryInitial = NewDuration(2 * time.Second)
			c.Kafka.RetryMax = NewDuration(time.Second)
		}},
		{name: "zero interval", change: func(c *Scheduler) { c.Scheduler.Interval = 0 }},
		{name: "zero batch", change: func(c *Scheduler) { c.Scheduler.BatchSize = 0 }},
		{name: "zero retention", change: func(c *Scheduler) { c.Scheduler.RetentionYears = 0 }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := valid
			test.change(&config)
			if err := config.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want an error")
			}
		})
	}
}

func TestStorerConfigRequiresConsumerGroup(t *testing.T) {
	config := NewStorer()
	config.Storage.DSN = "postgres://localhost/calendar"
	config.Kafka.GroupID = " "

	if err := config.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want an error")
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
