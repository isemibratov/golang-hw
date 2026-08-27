package kafka

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	segmentio "github.com/segmentio/kafka-go"
)

const testTopic = "calendar.notifications"

type topicCreatorResult struct {
	response *segmentio.CreateTopicsResponse
	err      error
}

type topicCreatorStub struct {
	results  []topicCreatorResult
	requests []*segmentio.CreateTopicsRequest
}

func (c *topicCreatorStub) CreateTopics(
	_ context.Context,
	request *segmentio.CreateTopicsRequest,
) (*segmentio.CreateTopicsResponse, error) {
	c.requests = append(c.requests, request)
	result := c.results[len(c.requests)-1]
	return result.response, result.err
}

func TestEnsureTopicCreatesAndProbesTopic(t *testing.T) {
	creator := newTopicCreatorStub(nil)
	probeCalls := 0

	err := ensureTopic(
		context.Background(),
		testTopic,
		time.Second,
		2*time.Second,
		nil,
		creator,
		func(context.Context) error {
			probeCalls++
			return nil
		},
		unexpectedRetryWait(t),
	)
	if err != nil {
		t.Fatalf("ensureTopic() error = %v", err)
	}
	if probeCalls != 1 {
		t.Fatalf("probe calls = %d, want 1", probeCalls)
	}
	assertTopicRequest(t, creator.requests, testTopic)
}

func TestEnsureTopicAcceptsExistingTopic(t *testing.T) {
	creator := newTopicCreatorStub(segmentio.TopicAlreadyExists)
	probeCalls := 0

	err := ensureTopic(
		context.Background(),
		testTopic,
		time.Second,
		2*time.Second,
		nil,
		creator,
		func(context.Context) error {
			probeCalls++
			return nil
		},
		unexpectedRetryWait(t),
	)
	if err != nil {
		t.Fatalf("ensureTopic() error = %v", err)
	}
	if probeCalls != 1 {
		t.Fatalf("probe calls = %d, want 1", probeCalls)
	}
}

func TestEnsureTopicStopsOnPermanentProtocolError(t *testing.T) {
	creator := newTopicCreatorStub(segmentio.TopicAuthorizationFailed)
	probeCalls := 0
	waitCalls := 0

	err := ensureTopic(
		context.Background(),
		testTopic,
		time.Second,
		2*time.Second,
		nil,
		creator,
		func(context.Context) error {
			probeCalls++
			return nil
		},
		func(context.Context, time.Duration) error {
			waitCalls++
			return nil
		},
	)
	if !errors.Is(err, segmentio.TopicAuthorizationFailed) {
		t.Fatalf("ensureTopic() error = %v, want %v", err, segmentio.TopicAuthorizationFailed)
	}
	if len(creator.requests) != 1 || probeCalls != 0 || waitCalls != 0 {
		t.Fatalf(
			"calls = (create %d, probe %d, wait %d), want (1, 0, 0)",
			len(creator.requests),
			probeCalls,
			waitCalls,
		)
	}
}

func TestEnsureTopicRetriesNetworkAndLeaderErrors(t *testing.T) {
	creator := &topicCreatorStub{results: []topicCreatorResult{
		{err: syscall.ECONNREFUSED},
		{response: topicResponse(nil)},
		{response: topicResponse(nil)},
	}}
	probeErrors := []error{segmentio.LeaderNotAvailable, nil}
	probeCalls := 0
	waits := make([]time.Duration, 0, 2)

	err := ensureTopic(
		context.Background(),
		testTopic,
		time.Second,
		2*time.Second,
		nil,
		creator,
		func(context.Context) error {
			err := probeErrors[probeCalls]
			probeCalls++
			return err
		},
		func(_ context.Context, delay time.Duration) error {
			waits = append(waits, delay)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("ensureTopic() error = %v", err)
	}
	if len(creator.requests) != 3 || probeCalls != 2 {
		t.Fatalf("calls = (create %d, probe %d), want (3, 2)", len(creator.requests), probeCalls)
	}
	wantWaits := []time.Duration{time.Second, 2 * time.Second}
	if !reflect.DeepEqual(waits, wantWaits) {
		t.Fatalf("retry waits = %v, want %v", waits, wantWaits)
	}
}

func TestEnsureTopicStopsActiveProbeOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	creator := newTopicCreatorStub(nil)
	waitCalls := 0

	err := ensureTopic(
		ctx,
		testTopic,
		time.Second,
		2*time.Second,
		nil,
		creator,
		func(ctx context.Context) error {
			cancel()
			<-ctx.Done()
			return ctx.Err()
		},
		func(context.Context, time.Duration) error {
			waitCalls++
			return nil
		},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ensureTopic() error = %v, want %v", err, context.Canceled)
	}
	if waitCalls != 0 {
		t.Fatalf("retry wait calls = %d, want 0", waitCalls)
	}
}

func TestEnsureTopicRejectsInvalidTopicBeforeConnecting(t *testing.T) {
	tests := []string{
		"",
		" ",
		".",
		"..",
		"calendar/notifications",
		"календарь",
		strings.Repeat("a", maxTopicNameLength+1),
	}

	for _, topic := range tests {
		t.Run(topic, func(t *testing.T) {
			err := EnsureTopic(
				context.Background(),
				[]string{testBroker},
				topic,
				time.Second,
				time.Second,
				2*time.Second,
				nil,
			)
			if err == nil {
				t.Fatal("EnsureTopic() error = nil, want an error")
			}
		})
	}
}

func newTopicCreatorStub(topicErr error) *topicCreatorStub {
	return &topicCreatorStub{results: []topicCreatorResult{{response: topicResponse(topicErr)}}}
}

func topicResponse(topicErr error) *segmentio.CreateTopicsResponse {
	return &segmentio.CreateTopicsResponse{Errors: map[string]error{testTopic: topicErr}}
}

func unexpectedRetryWait(t *testing.T) retryWait {
	t.Helper()
	return func(context.Context, time.Duration) error {
		t.Fatal("unexpected retry wait")
		return nil
	}
}

func assertTopicRequest(t *testing.T, requests []*segmentio.CreateTopicsRequest, topic string) {
	t.Helper()
	if len(requests) != 1 || len(requests[0].Topics) != 1 {
		t.Fatalf("CreateTopics requests = %#v, want one request with one topic", requests)
	}
	config := requests[0].Topics[0]
	if config.Topic != topic ||
		config.NumPartitions != topicPartitions ||
		config.ReplicationFactor != topicReplicationFactor {
		t.Fatalf("topic config = %#v, want topic %q with one partition and replica", config, topic)
	}
}
