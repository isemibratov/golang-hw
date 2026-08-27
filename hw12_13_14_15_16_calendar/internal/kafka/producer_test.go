package kafka

import (
	"context"
	"errors"
	"testing"
	"time"

	segmentio "github.com/segmentio/kafka-go"
)

type writerStub struct {
	messages   []segmentio.Message
	writeErr   error
	closeErr   error
	writeCalls int
	closed     bool
}

func (w *writerStub) WriteMessages(_ context.Context, messages ...segmentio.Message) error {
	w.writeCalls++
	w.messages = append(w.messages, messages...)
	return w.writeErr
}

func (w *writerStub) Close() error {
	w.closed = true
	return w.closeErr
}

func TestNewProducerConfiguresSynchronousWriter(t *testing.T) {
	producer := NewProducer([]string{"kafka-1:9092", "kafka-2:9092"}, "notifications", 4*time.Second)
	writer, ok := producer.writer.(*segmentio.Writer)
	if !ok {
		t.Fatalf("writer type = %T, want *kafka.Writer", producer.writer)
	}

	if writer.Topic != "notifications" {
		t.Fatalf("Topic = %q, want notifications", writer.Topic)
	}
	if writer.BatchSize != 1 {
		t.Fatalf("BatchSize = %d, want 1", writer.BatchSize)
	}
	if writer.WriteTimeout != 4*time.Second {
		t.Fatalf("WriteTimeout = %s, want 4s", writer.WriteTimeout)
	}
	if writer.RequiredAcks != segmentio.RequireAll {
		t.Fatalf("RequiredAcks = %d, want %d", writer.RequiredAcks, segmentio.RequireAll)
	}
	if writer.Async {
		t.Fatal("Async = true, want false")
	}
	if writer.AllowAutoTopicCreation {
		t.Fatal("AllowAutoTopicCreation = true, want false")
	}

	balancer, ok := writer.Balancer.(*segmentio.Hash)
	if !ok {
		t.Fatalf("Balancer type = %T, want *kafka.Hash", writer.Balancer)
	}
	partitions := []int{0, 1, 2, 3}
	message := segmentio.Message{Key: []byte("event-1")}
	first := balancer.Balance(message, partitions...)
	second := balancer.Balance(message, partitions...)
	if first != second {
		t.Fatalf("same key partitions = %d and %d, want equal", first, second)
	}
}

func TestProducerSend(t *testing.T) {
	writer := &writerStub{}
	producer := &Producer{writer: writer}
	wantKey := []byte("event-1")
	wantValue := []byte(`{"event_id":"event-1"}`)

	if err := producer.Send(context.Background(), wantKey, wantValue); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if len(writer.messages) != 1 {
		t.Fatalf("messages count = %d, want 1", len(writer.messages))
	}
	if string(writer.messages[0].Key) != string(wantKey) {
		t.Fatalf("message key = %q, want %q", writer.messages[0].Key, wantKey)
	}
	if string(writer.messages[0].Value) != string(wantValue) {
		t.Fatalf("message value = %q, want %q", writer.messages[0].Value, wantValue)
	}
}

func TestProducerSendReturnsWriterError(t *testing.T) {
	wantErr := errors.New("write failure")
	producer := &Producer{writer: &writerStub{writeErr: wantErr}}

	err := producer.Send(context.Background(), []byte("key"), []byte("value"))
	if !errors.Is(err, wantErr) {
		t.Fatalf("Send() error = %v, want %v", err, wantErr)
	}
}

func TestProducerSendHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	writer := &writerStub{}
	producer := &Producer{writer: writer}

	err := producer.Send(ctx, []byte("key"), []byte("value"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Send() error = %v, want %v", err, context.Canceled)
	}
	if writer.writeCalls != 0 {
		t.Fatalf("WriteMessages() calls = %d, want 0", writer.writeCalls)
	}
}

func TestProducerClose(t *testing.T) {
	wantErr := errors.New("close failure")
	writer := &writerStub{closeErr: wantErr}
	producer := &Producer{writer: writer}

	err := producer.Close()
	if !errors.Is(err, wantErr) {
		t.Fatalf("Close() error = %v, want %v", err, wantErr)
	}
	if !writer.closed {
		t.Fatal("writer was not closed")
	}
}
