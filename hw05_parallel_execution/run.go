package hw05parallelexecution

import (
	"errors"
	"sync"
)

var ErrErrorsLimitExceeded = errors.New("errors limit exceeded")

type Task func() error

// Run starts tasks in n goroutines and stops its work when receiving m errors from tasks.
// A non-positive n is treated as one worker; a non-positive m disables the errors limit.
func Run(tasks []Task, n, m int) error {
	if n <= 0 {
		n = 1
	}

	// The channels are unbuffered so a worker cannot take another task until
	// the coordinator has counted the result of its previous task.
	taskChannel := make(chan Task)
	resultChannel := make(chan error)

	var workers sync.WaitGroup
	workers.Add(n)
	for i := 0; i < n; i++ {
		go runWorker(taskChannel, resultChannel, &workers)
	}

	nextTaskIndex := 0
	runningTasks := 0
	errorsCount := 0
	errorsLimitExceeded := false

	for runningTasks > 0 || (!errorsLimitExceeded && nextTaskIndex < len(tasks)) {
		var (
			nextTask   Task
			taskOutput chan<- Task
		)
		// A nil output channel disables dispatching while the loop collects
		// results from tasks which had already started before the limit was reached.
		if !errorsLimitExceeded && nextTaskIndex < len(tasks) {
			nextTask = tasks[nextTaskIndex]
			taskOutput = taskChannel
		}

		select {
		case taskOutput <- nextTask:
			nextTaskIndex++
			runningTasks++
		case taskErr := <-resultChannel:
			runningTasks--
			if taskErr != nil && m > 0 {
				errorsCount++
				errorsLimitExceeded = errorsCount >= m
			}
		}
	}

	close(taskChannel)
	workers.Wait()

	if errorsLimitExceeded {
		return ErrErrorsLimitExceeded
	}

	return nil
}

func runWorker(tasks <-chan Task, results chan<- error, workers *sync.WaitGroup) {
	defer workers.Done()

	for task := range tasks {
		results <- task()
	}
}
