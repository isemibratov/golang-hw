package hw06pipelineexecution

type (
	In  = <-chan interface{}
	Out = In
	Bi  = chan interface{}
)

type Stage func(in In) (out Out)

func ExecutePipeline(in In, done In, stages ...Stage) Out {
	// The caller owns the pipeline input, so cancellation must not consume it.
	current := forward(in, done, false)
	for _, stage := range stages {
		// Drain stage outputs on cancellation to unblock in-flight sends.
		current = forward(stage(current), done, true)
	}

	return current
}

func forward(in In, done In, drainOnDone bool) Out {
	out := make(Bi)

	go func() {
		stop := func() {
			close(out)
			if drainOnDone && in != nil {
				for range in {
				}
			}
		}

		for {
			select {
			case <-done:
				stop()
				return
			default:
			}

			select {
			case <-done:
				stop()
				return
			case value, ok := <-in:
				if !ok {
					close(out)
					return
				}

				select {
				case <-done:
					stop()
					return
				case out <- value:
				}
			}
		}
	}()

	return out
}
