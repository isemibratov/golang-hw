package kafka

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

const testBroker = "kafka:9092"

type loggerStub struct {
	messages []string
}

func (l *loggerStub) Error(message string) {
	l.messages = append(l.messages, message)
}

func TestRetryWithBackoffUsesCappedDelay(t *testing.T) {
	attempts := 0
	waits := make([]time.Duration, 0, 4)
	logger := &loggerStub{}

	err := retryWithBackoff(
		context.Background(),
		time.Second,
		4*time.Second,
		logger,
		"Kafka topic is unavailable",
		func(context.Context) error {
			attempts++
			if attempts == 5 {
				return nil
			}
			return errors.New("broker unavailable")
		},
		func(_ context.Context, delay time.Duration) error {
			waits = append(waits, delay)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("retryWithBackoff() error = %v", err)
	}

	wantWaits := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 4 * time.Second}
	if !reflect.DeepEqual(waits, wantWaits) {
		t.Fatalf("retry waits = %v, want %v", waits, wantWaits)
	}
	if attempts != 5 {
		t.Fatalf("operation calls = %d, want 5", attempts)
	}
	if len(logger.messages) != 4 {
		t.Fatalf("log messages count = %d, want 4", len(logger.messages))
	}
}

func TestValidateRetryConfig(t *testing.T) {
	tests := []struct {
		name         string
		brokers      []string
		dialTimeout  time.Duration
		retryInitial time.Duration
		retryMax     time.Duration
	}{
		{name: "no brokers", dialTimeout: time.Second, retryInitial: time.Second, retryMax: time.Second},
		{
			name:         "blank broker",
			brokers:      []string{" "},
			dialTimeout:  time.Second,
			retryInitial: time.Second,
			retryMax:     time.Second,
		},
		{
			name:         "dial timeout",
			brokers:      []string{testBroker},
			retryInitial: time.Second,
			retryMax:     time.Second,
		},
		{
			name:        "initial backoff",
			brokers:     []string{testBroker},
			dialTimeout: time.Second,
			retryMax:    time.Second,
		},
		{
			name:         "maximum backoff",
			brokers:      []string{testBroker},
			dialTimeout:  time.Second,
			retryInitial: 2 * time.Second,
			retryMax:     time.Second,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateRetryConfig(
				test.brokers,
				test.dialTimeout,
				test.retryInitial,
				test.retryMax,
			)
			if err == nil {
				t.Fatal("validateRetryConfig() error = nil, want an error")
			}
		})
	}
}

func TestNextBackoffDoesNotOverflow(t *testing.T) {
	maximum := time.Duration(1<<63 - 1)
	current := maximum - time.Second
	if got := nextBackoff(current, maximum); got != maximum {
		t.Fatalf("nextBackoff() = %s, want %s", got, maximum)
	}
}
