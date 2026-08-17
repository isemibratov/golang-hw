package hw05parallelexecution

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestRun(t *testing.T) {
	defer goleak.VerifyNone(t)

	t.Run("stops after reaching the errors limit", func(t *testing.T) {
		const (
			tasksCount     = 50
			workersCount   = 10
			maxErrorsCount = 23
		)

		var runTasksCount int32
		tasks := make([]Task, 0, tasksCount)
		for i := 0; i < tasksCount; i++ {
			taskNumber := i
			tasks = append(tasks, func() error {
				atomic.AddInt32(&runTasksCount, 1)
				return fmt.Errorf("error from task %d", taskNumber)
			})
		}

		err := Run(tasks, workersCount, maxErrorsCount)

		require.ErrorIs(t, err, ErrErrorsLimitExceeded)
		require.GreaterOrEqual(t, runTasksCount, int32(maxErrorsCount))
		require.LessOrEqual(t, runTasksCount, int32(workersCount+maxErrorsCount), "extra tasks were started")
	})

	t.Run("runs all tasks when there are no errors", func(t *testing.T) {
		const tasksCount = 50

		var runTasksCount int32
		tasks := make([]Task, 0, tasksCount)
		for i := 0; i < tasksCount; i++ {
			tasks = append(tasks, func() error {
				atomic.AddInt32(&runTasksCount, 1)
				return nil
			})
		}

		err := Run(tasks, 5, 1)

		require.NoError(t, err)
		require.Equal(t, int32(tasksCount), runTasksCount, "not all tasks were completed")
	})

	t.Run("returns nil when errors limit is not reached", func(t *testing.T) {
		taskErr := errors.New("task error")
		var runTasksCount int32
		tasks := []Task{
			func() error { atomic.AddInt32(&runTasksCount, 1); return taskErr },
			func() error { atomic.AddInt32(&runTasksCount, 1); return nil },
			func() error { atomic.AddInt32(&runTasksCount, 1); return taskErr },
			func() error { atomic.AddInt32(&runTasksCount, 1); return nil },
		}

		err := Run(tasks, 2, 3)

		require.NoError(t, err)
		require.Equal(t, int32(len(tasks)), runTasksCount)
	})

	t.Run("ignores errors when the limit is non-positive", func(t *testing.T) {
		const tasksCount = 20

		var runTasksCount int32
		tasks := make([]Task, 0, tasksCount)
		for i := 0; i < tasksCount; i++ {
			tasks = append(tasks, func() error {
				atomic.AddInt32(&runTasksCount, 1)
				return errors.New("task error")
			})
		}

		err := Run(tasks, 4, 0)

		require.NoError(t, err)
		require.Equal(t, int32(tasksCount), runTasksCount)
	})

	t.Run("supports fewer tasks than workers", func(t *testing.T) {
		var runTasksCount int32
		tasks := []Task{
			func() error { atomic.AddInt32(&runTasksCount, 1); return nil },
			func() error { atomic.AddInt32(&runTasksCount, 1); return nil },
		}

		err := Run(tasks, 5, 1)

		require.NoError(t, err)
		require.Equal(t, int32(len(tasks)), runTasksCount)
	})

	t.Run("supports an empty task list", func(t *testing.T) {
		require.NoError(t, Run(nil, 3, 1))
	})

	t.Run("uses one worker when workers count is non-positive", func(t *testing.T) {
		var runTasksCount int32
		tasks := []Task{
			func() error { atomic.AddInt32(&runTasksCount, 1); return nil },
			func() error { atomic.AddInt32(&runTasksCount, 1); return nil },
		}

		err := Run(tasks, 0, 1)

		require.NoError(t, err)
		require.Equal(t, int32(len(tasks)), runTasksCount)
	})
}

func TestRunExecutesTasksConcurrently(t *testing.T) {
	defer goleak.VerifyNone(t)

	const workersCount = 4

	var startedTasks int32
	releaseTasks := make(chan struct{})
	tasks := make([]Task, workersCount)
	for i := range tasks {
		tasks[i] = func() error {
			atomic.AddInt32(&startedTasks, 1)
			<-releaseTasks
			return nil
		}
	}

	runDone := make(chan error, 1)
	go func() {
		runDone <- Run(tasks, workersCount, 1)
	}()

	var (
		releaseOnce sync.Once
		waitOnce    sync.Once
		runErr      error
	)
	release := func() {
		releaseOnce.Do(func() { close(releaseTasks) })
	}
	wait := func() {
		waitOnce.Do(func() { runErr = <-runDone })
	}
	defer func() {
		release()
		wait()
	}()

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&startedTasks) == workersCount
	}, time.Second, time.Millisecond)

	release()
	wait()
	require.NoError(t, runErr)
}
