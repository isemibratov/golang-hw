package hw06pipelineexecution

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	sleepPerStage = time.Millisecond * 100
	fault         = sleepPerStage / 2
	testTimeout   = 3 * time.Second
)

func TestPipeline(t *testing.T) {
	// Stage generator
	g := func(_ string, f func(v interface{}) interface{}) Stage {
		return func(in In) Out {
			out := make(Bi)
			go func() {
				defer close(out)
				for v := range in {
					time.Sleep(sleepPerStage)
					out <- f(v)
				}
			}()
			return out
		}
	}

	stages := []Stage{
		g("Dummy", func(v interface{}) interface{} { return v }),
		g("Multiplier (* 2)", func(v interface{}) interface{} { return v.(int) * 2 }),
		g("Adder (+ 100)", func(v interface{}) interface{} { return v.(int) + 100 }),
		g("Stringifier", func(v interface{}) interface{} { return strconv.Itoa(v.(int)) }),
	}

	t.Run("simple case", func(t *testing.T) {
		in := make(Bi)
		data := []int{1, 2, 3, 4, 5}

		go func() {
			for _, v := range data {
				in <- v
			}
			close(in)
		}()

		result := make([]string, 0, 10)
		start := time.Now()
		for s := range ExecutePipeline(in, nil, stages...) {
			result = append(result, s.(string))
		}
		elapsed := time.Since(start)

		require.Equal(t, []string{"102", "104", "106", "108", "110"}, result)
		require.Less(t,
			int64(elapsed),
			// ~0.8s for processing 5 values in 4 stages (100ms every) concurrently
			int64(sleepPerStage)*int64(len(stages)+len(data)-1)+int64(fault))
	})

	t.Run("done case", func(t *testing.T) {
		in := make(Bi)
		done := make(Bi)
		producerStopped := make(chan struct{})
		data := []int{1, 2, 3, 4, 5}

		// Abort after 200ms
		abortDur := sleepPerStage * 2
		go func() {
			<-time.After(abortDur)
			close(done)
		}()

		go func() {
			defer close(producerStopped)
			defer close(in)

			for _, v := range data {
				select {
				case <-done:
					return
				case in <- v:
				}
			}
		}()

		result := make([]string, 0, 10)
		start := time.Now()
		for s := range ExecutePipeline(in, done, stages...) {
			result = append(result, s.(string))
		}
		elapsed := time.Since(start)

		requireSignal(t, producerStopped, "producer did not stop")
		require.Len(t, result, 0)
		require.Less(t, int64(elapsed), int64(abortDur)+int64(fault))
	})
}

func TestExecutePipelineWithoutStages(t *testing.T) {
	t.Run("passes input values", func(t *testing.T) {
		in := make(Bi, 3)
		in <- 1
		in <- 2
		in <- 3
		close(in)

		var result []interface{}
		for value := range ExecutePipeline(in, nil) {
			result = append(result, value)
		}

		require.Equal(t, []interface{}{1, 2, 3}, result)
	})

	t.Run("stops on done", func(t *testing.T) {
		in := make(Bi)
		done := make(Bi)
		close(done)

		requireChannelClosed(t, ExecutePipeline(in, done))
	})
}

func TestExecutePipelineClosesStageInputOnDone(t *testing.T) {
	in := make(Bi)
	done := make(Bi)
	stageStopped := make(chan struct{})

	stage := func(in In) Out {
		out := make(Bi)
		go func() {
			defer close(out)
			for range in {
			}
			close(stageStopped)
		}()
		return out
	}

	out := ExecutePipeline(in, done, stage)
	close(done)

	requireSignal(t, stageStopped, "stage input was not closed")
	requireChannelClosed(t, out)
}

func TestExecutePipelineDrainsStageOutputOnDone(t *testing.T) {
	in := make(Bi, 2)
	in <- 1
	in <- 2
	close(in)

	done := make(Bi)
	secondSendStarted := make(chan struct{})
	stageStopped := make(chan struct{})

	stage := func(in In) Out {
		out := make(Bi)
		go func() {
			defer close(stageStopped)
			defer close(out)

			valuesSent := 0
			for value := range in {
				valuesSent++
				if valuesSent == 2 {
					close(secondSendStarted)
				}
				out <- value
			}
		}()
		return out
	}

	out := ExecutePipeline(in, done, stage)

	requireSignal(t, secondSendStarted, "stage did not start the second send")

	close(done)
	requireSignal(t, stageStopped, "stage remained blocked after pipeline cancellation")
	requireChannelClosed(t, out)
}

func requireSignal(t *testing.T, signal <-chan struct{}, failureMessage string) {
	t.Helper()

	select {
	case <-signal:
	case <-time.After(testTimeout):
		t.Fatal(failureMessage)
	}
}

func requireChannelClosed(t *testing.T, channel In) {
	t.Helper()

	select {
	case _, ok := <-channel:
		require.False(t, ok)
	case <-time.After(testTimeout):
		t.Fatal("channel was not closed")
	}
}
