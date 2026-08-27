package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const testTimeout = 3 * time.Second

func TestRunTransfersDataUntilInputEOF(t *testing.T) {
	const request = "from client\n"
	const response = "from server\n"

	listener := listen(t)
	input, inputWriter := io.Pipe()
	t.Cleanup(func() { _ = inputWriter.Close() })
	output := newObservedBuffer(len(response))
	var stderr bytes.Buffer
	args := addressArgs(t, listener)

	runDone := make(chan error, 1)
	go func() {
		runDone <- run(args, input, output, &stderr)
	}()

	connection := accept(t, listener)
	t.Cleanup(func() { _ = connection.Close() })

	_, err := connection.Write([]byte(response))
	require.NoError(t, err)
	wait(t, output.complete)

	require.NoError(t, wait(t, writeAsync(inputWriter, []byte(request))))
	require.NoError(t, inputWriter.Close())
	received, err := io.ReadAll(connection)
	require.NoError(t, err)
	require.Equal(t, request, string(received))

	require.NoError(t, wait(t, runDone))
	require.Equal(t, response, output.String())
	require.Contains(t, stderr.String(), "...EOF")
}

func TestRunStopsWhenPeerCloses(t *testing.T) {
	listener := listen(t)
	input, inputWriter := io.Pipe()
	t.Cleanup(func() { _ = inputWriter.Close() })
	var stderr bytes.Buffer
	args := addressArgs(t, listener)

	runDone := make(chan error, 1)
	go func() {
		runDone <- run(args, input, io.Discard, &stderr)
	}()

	connection := accept(t, listener)
	require.NoError(t, connection.Close())
	require.NoError(t, wait(t, runDone))
	require.Contains(t, stderr.String(), "...Connection was closed by peer")
	require.Error(t, wait(t, writeAsync(inputWriter, []byte("data"))))
}

func TestRunSessionStopsOnCancellation(t *testing.T) {
	listener := listen(t)
	input, inputWriter := io.Pipe()
	t.Cleanup(func() { _ = inputWriter.Close() })
	output := newBlockingWriter()
	t.Cleanup(func() { close(output.release) })
	client := NewTelnetClient(listener.Addr().String(), time.Second, input, output)
	require.NoError(t, client.Connect())

	connection := accept(t, listener)
	t.Cleanup(func() { _ = connection.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	var stderr bytes.Buffer
	runDone := make(chan error, 1)
	go func() {
		runDone <- runSession(ctx, client, log.New(&stderr, "", 0))
	}()

	_, err := connection.Write([]byte("response"))
	require.NoError(t, err)
	wait(t, output.started)
	cancel()
	require.NoError(t, wait(t, runDone))
	require.Contains(t, stderr.String(), "...Interrupted")

	buffer := make([]byte, 1)
	readCount, readErr := connection.Read(buffer)
	require.Zero(t, readCount)
	require.Error(t, readErr)
	require.Error(t, wait(t, writeAsync(inputWriter, []byte("data"))))
}

func TestRunSessionReturnsTransferError(t *testing.T) {
	listener := listen(t)
	input, inputWriter := io.Pipe()
	t.Cleanup(func() { _ = inputWriter.Close() })
	expectedErr := errors.New("output failure")
	client := NewTelnetClient(listener.Addr().String(), time.Second, input, errorWriter{expectedErr})
	require.NoError(t, client.Connect())

	connection := accept(t, listener)
	t.Cleanup(func() { _ = connection.Close() })
	runDone := make(chan error, 1)
	go func() {
		runDone <- runSession(context.Background(), client, log.New(io.Discard, "", 0))
	}()

	_, err := connection.Write([]byte("response"))
	require.NoError(t, err)
	require.ErrorIs(t, wait(t, runDone), expectedErr)
}

func TestRunReturnsConnectionError(t *testing.T) {
	listener := listen(t)
	args := append([]string{"--timeout=100ms"}, addressArgs(t, listener)...)
	require.NoError(t, listener.Close())

	err := run(args, io.NopCloser(strings.NewReader("")), io.Discard, io.Discard)

	require.Error(t, err)
	require.Contains(t, err.Error(), "connect to")
}

func TestRunValidatesArguments(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		errText    string
		stderrText string
	}{
		{name: "missing port", args: []string{"localhost"}, errText: "usage"},
		{name: "extra argument", args: []string{"localhost", "4242", "extra"}, errText: "usage"},
		{
			name:       "invalid timeout",
			args:       []string{"--timeout=invalid", "localhost", "4242"},
			errText:    "parse arguments",
			stderrText: "invalid value",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stderr bytes.Buffer
			err := run(test.args, io.NopCloser(strings.NewReader("")), io.Discard, &stderr)

			require.Error(t, err)
			require.Contains(t, err.Error(), test.errText)
			if test.stderrText != "" {
				require.Contains(t, stderr.String(), test.stderrText)
			}
		})
	}
}

func listen(t *testing.T) *net.TCPListener {
	t.Helper()

	listener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.ParseIP("127.0.0.1")})
	require.NoError(t, err)
	require.NoError(t, listener.SetDeadline(time.Now().Add(testTimeout)))
	t.Cleanup(func() { _ = listener.Close() })
	return listener
}

func accept(t *testing.T, listener *net.TCPListener) *net.TCPConn {
	t.Helper()

	connection, err := listener.AcceptTCP()
	require.NoError(t, err)
	require.NoError(t, connection.SetDeadline(time.Now().Add(testTimeout)))
	return connection
}

func addressArgs(t *testing.T, listener net.Listener) []string {
	t.Helper()

	host, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	return []string{host, port}
}

func writeAsync(writer io.Writer, data []byte) <-chan error {
	result := make(chan error, 1)
	go func() {
		_, err := writer.Write(data)
		result <- err
	}()
	return result
}

func wait[T any](t *testing.T, result <-chan T) T {
	t.Helper()

	select {
	case value := <-result:
		return value
	case <-time.After(testTimeout):
		t.Fatal("timed out waiting for result")
		var zero T
		return zero
	}
}

type observedBuffer struct {
	buffer   bytes.Buffer
	expected int
	complete chan struct{}
	mutex    sync.Mutex
}

func newObservedBuffer(expected int) *observedBuffer {
	return &observedBuffer{expected: expected, complete: make(chan struct{}, 1)}
}

func (b *observedBuffer) Write(data []byte) (int, error) {
	b.mutex.Lock()
	written, err := b.buffer.Write(data)
	complete := b.buffer.Len() >= b.expected
	b.mutex.Unlock()
	if complete {
		select {
		case b.complete <- struct{}{}:
		default:
		}
	}
	return written, err
}

func (b *observedBuffer) String() string {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.buffer.String()
}

type errorWriter struct {
	err error
}

func (w errorWriter) Write([]byte) (int, error) {
	return 0, w.err
}

type blockingWriter struct {
	started chan struct{}
	release chan struct{}
}

func newBlockingWriter() *blockingWriter {
	return &blockingWriter{started: make(chan struct{}, 1), release: make(chan struct{})}
}

func (w *blockingWriter) Write(data []byte) (int, error) {
	select {
	case w.started <- struct{}{}:
	default:
	}
	<-w.release
	return len(data), nil
}
