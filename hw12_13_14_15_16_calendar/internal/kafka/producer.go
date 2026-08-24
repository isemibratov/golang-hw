// Package kafka contains Kafka adapters used by the calendar background processes.
package kafka

import (
	"context"
	"fmt"
	"time"

	segmentio "github.com/segmentio/kafka-go"
)

type messageWriter interface {
	WriteMessages(ctx context.Context, messages ...segmentio.Message) error
	Close() error
}

// Producer synchronously publishes messages to one Kafka topic.
type Producer struct {
	writer messageWriter
}

// NewProducer creates a synchronous Kafka producer.
func NewProducer(brokers []string, topic string, writeTimeout time.Duration) *Producer {
	return &Producer{
		writer: &segmentio.Writer{
			Addr:         segmentio.TCP(brokers...),
			Topic:        topic,
			Balancer:     &segmentio.Hash{},
			BatchSize:    1,
			WriteTimeout: writeTimeout,
			RequiredAcks: segmentio.RequireAll,
			Async:        false,
		},
	}
}

// Send publishes one keyed message and waits for Kafka to acknowledge it.
func (p *Producer) Send(ctx context.Context, key, value []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	message := segmentio.Message{Key: key, Value: value}
	if err := p.writer.WriteMessages(ctx, message); err != nil {
		return fmt.Errorf("write Kafka message: %w", err)
	}

	return nil
}

// Close flushes pending writes and releases producer resources.
func (p *Producer) Close() error {
	if err := p.writer.Close(); err != nil {
		return fmt.Errorf("close Kafka writer: %w", err)
	}

	return nil
}
