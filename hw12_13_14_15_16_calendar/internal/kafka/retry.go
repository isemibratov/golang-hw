package kafka

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Logger is the logging contract needed while preparing Kafka.
type Logger interface {
	Error(message string)
}

type retryWait func(context.Context, time.Duration) error

type retryOperation func(context.Context) error

type permanentRetryError struct {
	cause error
}

func (e *permanentRetryError) Error() string {
	return e.cause.Error()
}

func (e *permanentRetryError) Unwrap() error {
	return e.cause
}

func retryWithBackoff(
	ctx context.Context,
	retryInitial time.Duration,
	retryMax time.Duration,
	logger Logger,
	description string,
	operation retryOperation,
	wait retryWait,
) error {
	backoff := retryInitial
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		err := operation(ctx)
		if err == nil {
			return nil
		}
		if contextDone(ctx) {
			return ctx.Err()
		}
		var permanent *permanentRetryError
		if errors.As(err, &permanent) {
			return permanent.cause
		}

		if logger != nil {
			logger.Error(fmt.Sprintf("%s: %v; retrying in %s", description, err, backoff))
		}
		if err = wait(ctx, backoff); err != nil {
			return err
		}

		backoff = nextBackoff(backoff, retryMax)
	}
}

func validateRetryConfig(
	brokers []string,
	dialTimeout time.Duration,
	retryInitial time.Duration,
	retryMax time.Duration,
) error {
	if len(brokers) == 0 {
		return errors.New("kafka brokers are empty")
	}
	for _, broker := range brokers {
		if strings.TrimSpace(broker) == "" {
			return errors.New("kafka broker address is empty")
		}
	}
	if dialTimeout <= 0 {
		return errors.New("kafka dial timeout must be positive")
	}
	if retryInitial <= 0 {
		return errors.New("kafka initial retry backoff must be positive")
	}
	if retryMax < retryInitial {
		return errors.New("kafka maximum retry backoff must not be less than the initial backoff")
	}

	return nil
}

func withoutRetry(err error) error {
	return &permanentRetryError{cause: err}
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func nextBackoff(current, maximum time.Duration) time.Duration {
	if current >= maximum || current > maximum-current {
		return maximum
	}

	return current * 2
}
