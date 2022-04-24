package run

import (
	"context"
	"sync"
	"time"
)

// Instance represents a runnable instance.
type Instance struct {
	r    Runnable
	opts *options

	// runs and failedRuns keep track of the number of
	// successful and failed executions of a runnable respectively.
	// failedRuns may be reset after a successful execution,
	// depending on restart options.
	runs, failedRuns uint64

	once sync.Once
}

// Run runs an instance in a goroutine and returns a channel
// where any encountered (non-nil) errors are propagated.
// An instance can be run at most once,
// with subsequent attempts returning a nil channel.
func (i *Instance) Run(ctx context.Context) <-chan error {
	return i.run(ctx)
}

func (i *Instance) run(ctx context.Context) <-chan error {
	var errCh chan error

	i.once.Do(func() {
		var chanSize uint
		if i.opts != nil {
			chanSize = i.opts.errChanSize
		}
		errCh = make(chan error, chanSize)

		go i.runCh(ctx, errCh)
	})

	return errCh
}

// runCh controls the execution of an instance based on its options
// and propagates the returned errors to the provided channel.
func (i *Instance) runCh(ctx context.Context, errCh chan<- error) {
	defer close(errCh)
	// Defer recovery if the appropriate option is set.
	if i.opts.calm() {
		defer func() {
			if episode := recover(); episode != nil {
				errCh <- RunnablePanic{Value: episode}
			}
		}()
	}

	var err error
	var after time.Duration
	for rerun := true; rerun; rerun, after = i.rerun(err) {
		// Wait for timeout between executions.
		// Note: No delay on first execution,
		//   since initial timeout value is zero.
		select {
		case <-ctx.Done():
			if ctx.Err() != nil {
				errCh <- ctx.Err()
			}
			return
		case <-time.After(after):
		}

		// Anonymous function to allow for immediate execution
		// of deferred context cancellation.
		err = func() error {
			ctxt, cancel := i.withContextTimeout(ctx)
			defer cancel()

			return i.r.run(ctxt)
		}()
		if err != nil {
			errCh <- err
		}
	}
}

// rerun indicates whether a runnable should run again after termination
// according to its options, as well as the delay after which it will.
// It should be provided with the return value of the previous execution.
func (i *Instance) rerun(err error) (rerun bool, after time.Duration) {
	if i.opts == nil {
		return
	}

	switch err {
	case nil:
		// Account for the current execution.
		i.runs++
		// If applicable, reset failure count.
		if i.opts.restartable.restartOnError {
			i.failedRuns = 0
		}
		// Check recurrence options, since execution was successful.
		if i.opts.recurring.recur {
			rerun, after = true, i.opts.recurring.period
		}
		// Run limit makes sense only if recurring.
		cOpts := i.opts.constrained
		if cOpts.runLimit != 0 && i.runs >= cOpts.runLimit {
			return false, 0
		}
	default:
		// Account for the failed execution.
		i.failedRuns++
		// Only restart options are applicable after failed execution.
		if rOpts := i.opts.restartable; rOpts.restartOnError {
			failLimit := rOpts.restartLimit
			if failLimit == 0 || i.failedRuns < failLimit {
				return true, rOpts.backoff(i.failedRuns)
			}
		}
	}
	return
}

// withContextTimeout creates a child of the provided context,
// applying timeout if applicable,
// and returns it along with its cancellation function.
func (i *Instance) withContextTimeout(ctx context.Context) (
	context.Context, context.CancelFunc) {

	if i.opts != nil && i.opts.constrained.timeout != 0 {
		timeout := i.opts.constrained.timeout
		return context.WithTimeout(ctx, timeout)
	}
	return context.WithCancel(ctx)
}
