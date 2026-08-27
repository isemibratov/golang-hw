package kafka

import (
	"context"
	"errors"
	"fmt"

	segmentio "github.com/segmentio/kafka-go"
)

type messageReader interface {
	FetchMessage(ctx context.Context) (segmentio.Message, error)
	CommitMessages(ctx context.Context, messages ...segmentio.Message) error
	Close() error
}

// Consumer reads and explicitly acknowledges messages from one Kafka consumer group.
type Consumer struct {
	reader messageReader
}

// NewConsumer creates a Kafka consumer with synchronous offset commits.
func NewConsumer(brokers []string, topic, groupID string, maxBytes int) *Consumer {
	return &Consumer{
		reader: segmentio.NewReader(segmentio.ReaderConfig{
			Brokers:        brokers,
			Topic:          topic,
			GroupID:        groupID,
			StartOffset:    segmentio.FirstOffset,
			CommitInterval: 0,
			MaxBytes:       maxBytes,
		}),
	}
}

// Consume handles messages until the context is canceled or an operation fails.
// A message is committed only after its handler succeeds.
func (c *Consumer) Consume(ctx context.Context, handler func(context.Context, []byte) error) error {
	if handler == nil {
		return errors.New("kafka message handler is nil")
	}

	for {
		message, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if contextDone(ctx) {
				return nil
			}
			return fmt.Errorf("fetch Kafka message: %w", err)
		}

		if err = handler(ctx, message.Value); err != nil {
			if contextDone(ctx) {
				return nil
			}
			return fmt.Errorf("handle Kafka message: %w", err)
		}

		if err = c.reader.CommitMessages(ctx, message); err != nil {
			if contextDone(ctx) {
				return nil
			}
			return fmt.Errorf("commit Kafka message: %w", err)
		}
	}
}

// Close releases consumer resources.
func (c *Consumer) Close() error {
	if err := c.reader.Close(); err != nil {
		return fmt.Errorf("close Kafka reader: %w", err)
	}

	return nil
}

func contextDone(ctx context.Context) bool {
	return ctx.Err() != nil
}
