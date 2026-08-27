package kafka

import (
	"context"
	"errors"
	"reflect"
	"testing"

	segmentio "github.com/segmentio/kafka-go"
)

const testPayload = "payload"

func TestNewConsumerConfiguresReader(t *testing.T) {
	brokers := []string{"kafka-1:9092", "kafka-2:9092"}
	consumer := NewConsumer(brokers, "notifications", "calendar-storer", 2<<20)
	reader, ok := consumer.reader.(*segmentio.Reader)
	if !ok {
		t.Fatalf("reader type = %T, want *kafka.Reader", consumer.reader)
	}

	config := reader.Config()
	if !reflect.DeepEqual(config.Brokers, brokers) {
		t.Fatalf("Brokers = %v, want %v", config.Brokers, brokers)
	}
	if config.Topic != "notifications" {
		t.Fatalf("Topic = %q, want notifications", config.Topic)
	}
	if config.GroupID != "calendar-storer" {
		t.Fatalf("GroupID = %q, want calendar-storer", config.GroupID)
	}
	if config.StartOffset != segmentio.FirstOffset {
		t.Fatalf("StartOffset = %d, want %d", config.StartOffset, segmentio.FirstOffset)
	}
	if config.CommitInterval != 0 {
		t.Fatalf("CommitInterval = %s, want synchronous commits", config.CommitInterval)
	}
	if config.MaxBytes != 2<<20 {
		t.Fatalf("MaxBytes = %d, want %d", config.MaxBytes, 2<<20)
	}
}

type readerStub struct {
	messages    []segmentio.Message
	fetchErr    error
	commitErr   error
	closeErr    error
	cancel      context.CancelFunc
	sequence    *[]string
	committed   []segmentio.Message
	closed      bool
	fetchCursor int
}

func (r *readerStub) FetchMessage(ctx context.Context) (segmentio.Message, error) {
	if r.sequence != nil {
		*r.sequence = append(*r.sequence, "fetch")
	}
	if r.fetchCursor < len(r.messages) {
		message := r.messages[r.fetchCursor]
		r.fetchCursor++
		return message, nil
	}
	if r.fetchErr != nil {
		return segmentio.Message{}, r.fetchErr
	}

	<-ctx.Done()
	return segmentio.Message{}, ctx.Err()
}

func (r *readerStub) CommitMessages(_ context.Context, messages ...segmentio.Message) error {
	if r.sequence != nil {
		*r.sequence = append(*r.sequence, "commit")
	}
	r.committed = append(r.committed, messages...)
	if r.cancel != nil {
		r.cancel()
	}
	return r.commitErr
}

func (r *readerStub) Close() error {
	r.closed = true
	return r.closeErr
}

func TestConsumerHandlesBeforeCommit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	sequence := make([]string, 0, 4)
	message := segmentio.Message{Key: []byte("event-1"), Value: []byte(testPayload), Offset: 42}
	reader := &readerStub{
		messages: []segmentio.Message{message},
		cancel:   cancel,
		sequence: &sequence,
	}
	consumer := &Consumer{reader: reader}

	err := consumer.Consume(ctx, func(_ context.Context, value []byte) error {
		sequence = append(sequence, "handle")
		if string(value) != testPayload {
			t.Fatalf("handler value = %q, want payload", value)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Consume() error = %v", err)
	}

	wantSequence := []string{"fetch", "handle", "commit", "fetch"}
	if !reflect.DeepEqual(sequence, wantSequence) {
		t.Fatalf("sequence = %v, want %v", sequence, wantSequence)
	}
	if len(reader.committed) != 1 || reader.committed[0].Offset != message.Offset {
		t.Fatalf("committed messages = %+v, want offset %d", reader.committed, message.Offset)
	}
}

func TestConsumerDoesNotCommitHandlerFailure(t *testing.T) {
	wantErr := errors.New("handler failure")
	reader := &readerStub{messages: []segmentio.Message{{Value: []byte(testPayload)}}}
	consumer := &Consumer{reader: reader}

	err := consumer.Consume(context.Background(), func(context.Context, []byte) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Consume() error = %v, want %v", err, wantErr)
	}
	if len(reader.committed) != 0 {
		t.Fatalf("committed messages count = %d, want 0", len(reader.committed))
	}
}

func TestConsumerReturnsCommitFailure(t *testing.T) {
	wantErr := errors.New("commit failure")
	reader := &readerStub{
		messages:  []segmentio.Message{{Value: []byte(testPayload)}},
		commitErr: wantErr,
	}
	consumer := &Consumer{reader: reader}

	err := consumer.Consume(context.Background(), func(context.Context, []byte) error { return nil })
	if !errors.Is(err, wantErr) {
		t.Fatalf("Consume() error = %v, want %v", err, wantErr)
	}
	if len(reader.committed) != 1 {
		t.Fatalf("committed messages count = %d, want 1 attempt", len(reader.committed))
	}
}

func TestConsumerCancellationIsSuccessful(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	consumer := &Consumer{reader: &readerStub{}}

	if err := consumer.Consume(ctx, func(context.Context, []byte) error { return nil }); err != nil {
		t.Fatalf("Consume() error = %v, want nil", err)
	}
}

func TestConsumerRejectsNilHandler(t *testing.T) {
	consumer := &Consumer{reader: &readerStub{}}
	if err := consumer.Consume(context.Background(), nil); err == nil {
		t.Fatal("Consume() error = nil, want an error")
	}
}

func TestConsumerClose(t *testing.T) {
	wantErr := errors.New("close failure")
	reader := &readerStub{closeErr: wantErr}
	consumer := &Consumer{reader: reader}

	err := consumer.Close()
	if !errors.Is(err, wantErr) {
		t.Fatalf("Close() error = %v, want %v", err, wantErr)
	}
	if !reader.closed {
		t.Fatal("reader was not closed")
	}
}
