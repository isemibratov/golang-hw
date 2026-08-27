package kafka

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	segmentio "github.com/segmentio/kafka-go"
)

const (
	topicPartitions        = 1
	topicReplicationFactor = 1
	maxTopicNameLength     = 249
)

type topicCreator interface {
	CreateTopics(context.Context, *segmentio.CreateTopicsRequest) (*segmentio.CreateTopicsResponse, error)
}

type topicProbe func(context.Context) error

// EnsureTopic creates the notification topic when needed and waits for its leader.
func EnsureTopic(
	ctx context.Context,
	brokers []string,
	topic string,
	dialTimeout time.Duration,
	retryInitial time.Duration,
	retryMax time.Duration,
	logger Logger,
) error {
	if err := validateTopicName(topic); err != nil {
		return err
	}
	if err := validateRetryConfig(brokers, dialTimeout, retryInitial, retryMax); err != nil {
		return err
	}

	networkDialer := &net.Dialer{Timeout: dialTimeout}
	transport := &segmentio.Transport{Dial: networkDialer.DialContext}
	defer transport.CloseIdleConnections()
	client := &segmentio.Client{
		Addr:      segmentio.TCP(brokers...),
		Timeout:   dialTimeout,
		Transport: transport,
	}

	return ensureTopic(
		ctx,
		topic,
		retryInitial,
		retryMax,
		logger,
		client,
		func(ctx context.Context) error { return probeTopic(ctx, brokers, topic, dialTimeout) },
		waitForRetry,
	)
}

func ensureTopic(
	ctx context.Context,
	topic string,
	retryInitial time.Duration,
	retryMax time.Duration,
	logger Logger,
	client topicCreator,
	probe topicProbe,
	wait retryWait,
) error {
	return retryWithBackoff(
		ctx,
		retryInitial,
		retryMax,
		logger,
		"Kafka topic is unavailable",
		func(ctx context.Context) error {
			if err := createTopic(ctx, client, topic); err != nil {
				return classifyTopicError(err)
			}
			if err := probe(ctx); err != nil {
				return classifyTopicError(err)
			}
			return nil
		},
		wait,
	)
}

func createTopic(ctx context.Context, client topicCreator, topic string) error {
	response, err := client.CreateTopics(ctx, &segmentio.CreateTopicsRequest{
		Topics: []segmentio.TopicConfig{{
			Topic:             topic,
			NumPartitions:     topicPartitions,
			ReplicationFactor: topicReplicationFactor,
		}},
	})
	if err != nil {
		return fmt.Errorf("create Kafka topic: %w", err)
	}
	if response == nil {
		return errors.New("create Kafka topic: empty response")
	}
	if topicErr := response.Errors[topic]; topicErr != nil &&
		!errors.Is(topicErr, segmentio.TopicAlreadyExists) {
		return fmt.Errorf("create Kafka topic %q: %w", topic, topicErr)
	}
	return nil
}

func probeTopic(
	ctx context.Context,
	brokers []string,
	topic string,
	dialTimeout time.Duration,
) error {
	dialer := &segmentio.Dialer{Timeout: dialTimeout}
	var lastErr error
	for _, broker := range brokers {
		if err := ctx.Err(); err != nil {
			return err
		}
		attemptCtx, cancel := context.WithTimeout(ctx, dialTimeout)
		connection, err := dialer.DialLeader(attemptCtx, "tcp", broker, topic, 0)
		cancel()
		if err != nil {
			lastErr = err
			continue
		}
		if err = connection.Close(); err != nil {
			return fmt.Errorf("close Kafka topic probe connection: %w", err)
		}
		return nil
	}

	return fmt.Errorf("dial Kafka topic leader: %w", lastErr)
}

func classifyTopicError(err error) error {
	var protocolErr segmentio.Error
	if errors.As(err, &protocolErr) && !protocolErr.Temporary() {
		return withoutRetry(err)
	}
	return err
}

func validateTopicName(topic string) error {
	switch {
	case strings.TrimSpace(topic) == "":
		return errors.New("kafka topic is empty")
	case len(topic) > maxTopicNameLength:
		return fmt.Errorf("kafka topic exceeds %d bytes", maxTopicNameLength)
	case topic == "." || topic == "..":
		return fmt.Errorf("kafka topic %q is reserved", topic)
	}

	for index := 0; index < len(topic); index++ {
		if !isTopicNameByte(topic[index]) {
			return fmt.Errorf("kafka topic contains invalid character %q", topic[index])
		}
	}
	return nil
}

func isTopicNameByte(value byte) bool {
	return value >= 'a' && value <= 'z' ||
		value >= 'A' && value <= 'Z' ||
		value >= '0' && value <= '9' ||
		value == '.' || value == '_' || value == '-'
}
