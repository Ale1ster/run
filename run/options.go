package run

import "time"

// options encapsulates a runnable's execution options.
type options struct {
	errChanSize uint
	recurring   recurrenceOptions
	constrained constraintOptions
	restartable restartOptions
	recoverable panicOptions
}

// Option represents an execution option for a runnable.
type Option func(*options) *options

// WithChanBuffer controls the buffer size of the error channel
// (Default: unbuffered, equivalent to setting size to 0).
//
// Note that in case an error is returned with an unbuffered channel,
// it will result in the expected outcome of the runnable's execution
// waiting for the error to be received in order to resume.
func WithChanBuffer(sz uint) Option {
	return func(o *options) *options {
		o.errChanSize = sz
		return o
	}
}

// recurrenceOptions defines periodic options.
type recurrenceOptions struct {
	// recur denotes whether a runnable
	// should be rerun after successful execution.
	recur bool
	// period represents the amount of time between
	// a successful termination of a runnable and
	// the start of its next execution.
	period time.Duration
}

// Recur indicates whether to rerun a runnable after successful executions.
func Recur(recur bool) Option {
	return func(o *options) *options {
		o.recurring.recur = recur
		return o
	}
}

// Period sets the period between executions of a runnable.
func Period(period time.Duration) Option {
	return func(o *options) *options {
		o.recurring.period = period
		return o
	}
}

// constraintOptions defines execution constraint options.
type constraintOptions struct {
	// timeout is the maximum amount of time
	// a runnable can execute for (in a single run).
	timeout time.Duration
	// runLimit limits the amount of successful executions of a runnable.
	runLimit uint64
}

// Timeout sets the execution timeout for a runnable.
func Timeout(timeout time.Duration) Option {
	return func(o *options) *options {
		o.constrained.timeout = timeout
		return o
	}
}

// RunLimit sets the limit of successful executions for a runnable
// (applicable only when execution is recurring, with default value 0).
//
// Any failed executions do not count towards this limit.
// A value of 0 represents no limit.
func RunLimit(limit uint64) Option {
	return func(o *options) *options {
		o.constrained.runLimit = limit
		return o
	}
}

// BackoffFn represents the signature of a backoff function.
type BackoffFn func(count uint64) time.Duration

// ConstantBackoff returns a backoff function whose period
// is set to the provided duration.
func ConstantBackoff(d time.Duration) BackoffFn {
	return func(_ uint64) time.Duration {
		return d
	}
}

// restartOptions sets restart options
// for failed executions of a runnable (executions that terminated with error).
type restartOptions struct {
	// restartOnError indicates whether failed executions
	// can be can be restarted (limited by runLimit).
	restartOnError bool
	// restartLimit is the maximum numer of times
	// a runnable should be restarted, with 0 representing no limit.
	restartLimit uint64
	// resetOnSuccess indicates whether the failure count of a runnable
	// should be reset upon successful execution.
	resetOnSuccess bool
	// backoff determines the backoff period
	// after the n-th (continuous) failed execution of a runnable.
	// If unset, the runnable is restarted immediately after a failure.
	backoff BackoffFn
}

// Restart indicates whether to restart a runnable after failed executions.
func Restart(restartOnError bool) Option {
	return func(o *options) *options {
		o.restartable.restartOnError = restartOnError
		return o
	}
}

// RestartLimit sets a limit on the number of restarts
// after failed executions of a runnable, as well as
// a configurable backoff period after each one.
//
// A limit value of 0 represents no limit.
// If nil is provided as the backoff function, no backoff is applied.
func RestartLimit(limit uint64, backoffFn BackoffFn) Option {
	boff := ConstantBackoff(0)
	if backoffFn != nil {
		boff = backoffFn
	}

	return func(o *options) *options {
		o.restartable.restartLimit = limit
		o.restartable.backoff = boff
		return o
	}
}

// ResetOnSuccess resets the failure count of runnable
// upon successful execution (default: false).
//
// If this option is unset and a runnable has a restart limit n > 0,
// it will terminate upon reaching n total failed executions.
func ResetOnSuccess(reset bool) Option {
	return func(o *options) *options {
		o.restartable.resetOnSuccess = reset
		return o
	}
}

// panicOptions defines recovery options in case
// panic is encountered during a runnable's execution.
type panicOptions struct {
	// calm indicates whether panic during execution
	// should be recovered from and returned as an error.
	calm bool
}

// Recover allows a runnable to recover from a panic
// and return an error indicating the reason (default: false).
//
// Execution of the runnable is terminated upon panic,
// ignoring any restart options.
func Recover(r bool) Option {
	return func(o *options) *options {
		o.recoverable.calm = r
		return o
	}
}

// calm indicates whether the runnable should recover from panic.
func (o *options) calm() bool {
	return (o != nil) && o.recoverable.calm
}
