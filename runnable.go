package run

import "context"

// Runnable defines the contract for a runnable.
//
// It should respect context cancellation.
type Runnable func(context.Context) error

func (r Runnable) run(ctx context.Context) error {
	if r == nil {
		panic(NilRunnable)
	}

	return r(ctx)
}

// New creates a new runnable instance with the provided options.
//
// In case of conflicting options, the last one will be applied.
func New(r Runnable, opts ...Option) Instance {
	runnableOpts := new(options)
	for _, opt := range opts {
		runnableOpts = opt(runnableOpts)
	}

	return Instance{
		r:    r,
		opts: runnableOpts,
	}
}
