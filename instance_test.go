package run

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	testTimeDelta = 30 * time.Millisecond
)

func str(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}

type expectations struct {
	*assert.Assertions

	received []*mockCall

	expecting <-chan *mockCall
}

func (e *expectations) withAssertions(as *assert.Assertions) {
	e.Assertions = as
}

func (e *expectations) run(ctx context.Context) error {
	callTime := time.Now()

	// Verify call is expected
	if !e.NotEmpty(e.expecting, "unexpected call") {
		e.FailNow(str("expected %d calls, current call is no.%d",
			cap(e.expecting), len(e.received)+1))
	}

	current := <-e.expecting
	current._calledAt = callTime

	// Verify elapsed time (delay) after last call returned
	if len(e.received) > 0 {
		expectedCallTime := e.received[len(e.received)-1].
			_returnedAt.Add(current.expectedAfter)
		e.WithinDurationf(expectedCallTime, current._calledAt, testTimeDelta,
			str("unexpected call delay on execution no.%d", len(e.received)+1))
	}

	if current.argVerifier != nil {
		current.argVerifier(e.Assertions, ctx)
	}

	// Update call expectations
	defer func() {
		e.received = append(e.received, current)
		e.received[len(e.received)-1]._returnedAt = time.Now()
	}()

	// Simulate return delay
	if current.wait != 0 {
		<-time.After(current.wait)
	}

	if current.panics {
		panic(current.panicsWith)
	}

	return current.returnValue
}

func (e *expectations) verify() {
	e.Empty(e.expecting, "unexpected total number of calls")
}

func newExpectations(calls ...*mockCall) *expectations {
	expectedCalls := make(chan *mockCall, len(calls))
	for _, call := range calls {
		expectedCalls <- call
	}

	return &expectations{
		expecting: expectedCalls,
	}
}

type mockCall struct {
	expectedAfter time.Duration
	argVerifier   func(*assert.Assertions, context.Context)

	wait        time.Duration
	returnValue error
	panics      bool
	panicsWith  interface{}

	_calledAt, _returnedAt time.Time
}

// Creates a new expected call mock
func expect() *mockCall {
	return &mockCall{}
}

// Sets call return value
func (c *mockCall) returning(e error) *mockCall {
	c.returnValue = e
	return c
}

// Sets call panic value (takes precedence over return)
func (c *mockCall) panicking(v interface{}) *mockCall {
	c.panics = true
	c.panicsWith = v
	return c
}

// Sets call delay before returning (or panicking)
func (c *mockCall) after(waitFor time.Duration) *mockCall {
	c.wait = waitFor
	return c
}

// Sets delay expectation of call after last call returned
func (c *mockCall) withDelay(delay time.Duration) *mockCall {
	c.expectedAfter = delay
	return c
}

// Sets argument verification on call
func (c *mockCall) verifyArg(timeout time.Duration, values map[interface{}]interface{}) *mockCall {
	c.argVerifier = func(as *assert.Assertions, ctx context.Context) {
		// Verify timeout
		switch deadline, hasTimeout := ctx.Deadline(); hasTimeout {
		case true:
			expectedDeadline := c._calledAt.Add(timeout)
			as.WithinDurationf(expectedDeadline, deadline, testTimeDelta,
				"unexpected context deadline")
		default:
			as.Zero(timeout, "unexpected context without deadline")
		}
		// Verify values
		if len(values) != 0 {
			receivedVals := make(map[interface{}]interface{}, len(values))
			for key := range values {
				receivedVals[key] = ctx.Value(key)
			}
			as.EqualValues(values, receivedVals,
				"missing or differing context values")
		}
	}
	return c
}

func testInstance(t *testing.T) {
	type testcase struct {
		long bool

		expect *expectations
		opts   *options

		contextTimeout time.Duration
		contextValues  map[interface{}]interface{}

		errorReducer   func(<-chan error) []error
		expectedErrors []error
	}

	ctxTestVals := map[interface{}]interface{}{
		"a": 42,
		3:   "b",
	}

	var (
		testWaitTime    = 81 * time.Millisecond
		testRunPeriod   = 114 * time.Millisecond
		testBackoffStep = 287 * time.Millisecond
	)

	subtests := map[string]testcase{
		"run calls runnable": testcase{
			expect: newExpectations(
				expect().returning(nil),
			),
			opts:           &options{},
			expectedErrors: []error{},
		},
		"nil options": testcase{
			expect: newExpectations(
				expect().returning(nil),
			),
			opts:           nil,
			expectedErrors: []error{},
		},
		"recurring without error runs until limit": testcase{
			expect: newExpectations(
				expect().returning(nil),
				expect().returning(nil),
				expect().returning(nil),
				expect().returning(nil),
				expect().returning(nil),
			),
			opts: &options{
				recurring: recurrenceOptions{
					recur: true,
				},
				constrained: constraintOptions{
					runLimit: 5,
				},
			},
			expectedErrors: []error{},
		},
		"recurring stops on error": testcase{
			expect: newExpectations(
				expect().returning(nil),
				expect().returning(nil),
				expect().returning(testError(1)),
			),
			opts: &options{
				recurring: recurrenceOptions{
					recur: true,
				},
				constrained: constraintOptions{
					runLimit: 5,
				},
			},
			expectedErrors: []error{
				testError(1),
			},
		},
		"restartable with error runs until limit": testcase{
			expect: newExpectations(
				expect().returning(testError(1)),
				expect().returning(testError(2)),
				expect().returning(testError(3)),
			),
			opts: &options{
				restartable: restartOptions{
					restartOnError: true,
					restartLimit:   3,
					backoff:        ConstantBackoff(0),
				},
			},
			expectedErrors: []error{
				testError(1), testError(2), testError(3),
			},
		},
		"restartable stops on success": testcase{
			expect: newExpectations(
				expect().returning(nil),
			),
			opts: &options{
				restartable: restartOptions{
					restartOnError: true,
					restartLimit:   3,
					backoff:        ConstantBackoff(0),
				},
			},
			expectedErrors: []error{},
		},
		"recurring restartable runs until limit successes": testcase{
			expect: newExpectations(
				expect().returning(nil),
				expect().returning(testError(1)),
				expect().returning(nil),
				expect().returning(nil),
			),
			opts: &options{
				recurring: recurrenceOptions{
					recur: true,
				},
				constrained: constraintOptions{
					runLimit: 3,
				},
				restartable: restartOptions{
					restartOnError: true,
					restartLimit:   2,
					backoff:        ConstantBackoff(0),
				},
			},
			expectedErrors: []error{
				testError(1),
			},
		},
		"recurring restartable with reset resets on success": testcase{
			expect: newExpectations(
				expect().returning(nil),
				expect().returning(testError(1)),
				expect().returning(nil),
				expect().returning(testError(2)),
				expect().returning(nil),
				expect().returning(testError(3)),
				expect().returning(nil),
				expect().returning(nil),
			),
			opts: &options{
				recurring: recurrenceOptions{
					recur: true,
				},
				constrained: constraintOptions{
					runLimit: 5,
				},
				restartable: restartOptions{
					restartOnError: true,
					restartLimit:   3,
					backoff:        ConstantBackoff(0),
					resetOnSuccess: true,
				},
			},
			expectedErrors: []error{
				testError(1), testError(2), testError(3),
			},
		},
		"recoverable does not panic": testcase{
			expect: newExpectations(
				expect().panicking("panic message"),
			),
			opts: &options{
				recoverable: panicOptions{
					calm: true,
				},
			},
			expectedErrors: []error{
				RunnablePanic{"panic message"},
			},
		},
		"recurring recoverable stops on panic": testcase{
			expect: newExpectations(
				expect().returning(nil),
				expect().returning(nil),
				expect().panicking("panic message"),
			),
			opts: &options{
				recurring: recurrenceOptions{
					recur: true,
				},
				recoverable: panicOptions{
					calm: true,
				},
			},
			expectedErrors: []error{
				RunnablePanic{"panic message"},
			},
		},
		"runnable context contains parent values": testcase{
			expect: newExpectations(
				expect().returning(nil).verifyArg(0, ctxTestVals),
			),
			opts:           &options{},
			contextValues:  ctxTestVals,
			expectedErrors: []error{},
		},
		"runnable context with timeout": testcase{
			expect: newExpectations(
				expect().returning(nil).verifyArg(3*time.Second, nil),
			),
			opts: &options{
				constrained: constraintOptions{
					timeout: 3 * time.Second,
				},
			},
			expectedErrors: []error{},
		},
		"parent context deadline exceeded during backoff": testcase{
			long: true,
			expect: newExpectations(
				expect().returning(nil).after(testWaitTime),
				expect().returning(nil).after(testWaitTime),
				expect().returning(nil).after(testWaitTime),
				expect().returning(nil).after(testWaitTime),
				expect().returning(nil).after(testWaitTime),
				expect().returning(testError(1)),
			),
			opts: &options{
				recurring: recurrenceOptions{
					recur: true,
				},
				restartable: restartOptions{
					restartOnError: true,
					restartLimit:   3,
					backoff: func(_ uint64) time.Duration {
						return testBackoffStep
					},
				},
			},
			contextTimeout: 500 * time.Millisecond,
			expectedErrors: []error{
				testError(1),
				context.DeadlineExceeded,
			},
		},
		"recurring waits for period before next run on success": testcase{
			long: true,
			expect: newExpectations(
				expect().returning(nil),
				expect().returning(nil).withDelay(testRunPeriod),
				expect().returning(nil).withDelay(testRunPeriod),
			),
			opts: &options{
				recurring: recurrenceOptions{
					recur:  true,
					period: testRunPeriod,
				},
				constrained: constraintOptions{
					runLimit: 3,
				},
			},
			expectedErrors: []error{},
		},
		"restartable waits for backoff before next run on error": testcase{
			long: true,
			expect: newExpectations(
				expect().returning(testError(1)),
				expect().returning(testError(2)).withDelay(testBackoffStep),
				expect().returning(testError(3)).withDelay(2*testBackoffStep),
				expect().returning(testError(4)).withDelay(3*testBackoffStep),
			),
			opts: &options{
				restartable: restartOptions{
					restartOnError: true,
					restartLimit:   4,
					backoff: func(n uint64) time.Duration {
						return time.Duration(n) * testBackoffStep
					},
				},
			},
			expectedErrors: []error{
				testError(1), testError(2), testError(3), testError(4),
			},
		},
		"buffered error channel does not block": testcase{
			expect: newExpectations(
				expect().returning(nil).withDelay(0),
				expect().returning(testError(1)).withDelay(0),
				expect().returning(testError(2)).withDelay(0),
				expect().returning(testError(3)).withDelay(0),
				expect().returning(testError(4)).withDelay(0),
				expect().returning(testError(5)).withDelay(200*time.Millisecond),
				expect().returning(testError(6)).withDelay(0),
				expect().returning(nil).withDelay(0),
			),
			opts: &options{
				errChanSize: 2,
				recurring: recurrenceOptions{
					recur: true,
				},
				constrained: constraintOptions{
					runLimit: 2,
				},
				restartable: restartOptions{
					restartOnError: true,
					restartLimit:   7,
					backoff:        ConstantBackoff(0),
				},
			},
			errorReducer: func(errChan <-chan error) []error {
				errs := make([]error, 0)

				// Read the first error and delay before reading the rest.
				// That way, the runnable's execution will be delayed
				// after `errChanSize+1` erroneous runs later.
				// That is since, `errChanSize` results can be buffered,
				// and the next write will block until channel has space before returning.
				errs = append(errs, <-errChan)
				<-time.After(200 * time.Millisecond)

				for err := range errChan {
					errs = append(errs, err)
				}
				return errs
			},
			expectedErrors: []error{
				testError(1), testError(2), testError(3),
				testError(4), testError(5), testError(6),
			},
		},
	}

	for name, tc := range subtests {
		t.Run(name, func(t *testing.T) {
			if tc.long && testing.Short() {
				t.Skip()
			}

			as := newAssertions(t)

			ctx, cancel := prepareContext(tc.contextTimeout, tc.contextValues)
			defer cancel()

			tc.expect.withAssertions(as)
			defer tc.expect.verify()

			inst := Instance{
				r:    tc.expect.run,
				opts: tc.opts,
			}

			errorChan := inst.Run(ctx)

			reduceErrors := waitErrors
			if tc.errorReducer != nil {
				reduceErrors = tc.errorReducer
			}

			returned := reduceErrors(errorChan)
			as.Equalf(tc.expectedErrors, returned, "runnable returned unexpected errors")
		})
	}
}

func waitErrors(errorChan <-chan error) []error {
	errs := make([]error, 0)
	for err := range errorChan {
		errs = append(errs, err)
	}
	return errs
}

func prepareContext(timeout time.Duration,
	vals map[interface{}]interface{}) (context.Context, context.CancelFunc) {

	base := context.TODO()

	for k, v := range vals {
		base = context.WithValue(base, k, v)
	}

	switch timeout {
	case 0:
		return context.WithCancel(base)
	default:
		return context.WithTimeout(base, timeout)
	}
}

func testNew(t *testing.T) {
	subtests := map[string]func(*testing.T){
		"nil runnable": func(t *testing.T) {
			as := newAssertions(t)

			var runner Runnable
			inst := New(runner)

			as.Nil(inst.r)
		},
		"no options": func(t *testing.T) {
			as := newAssertions(t)

			var runner Runnable
			inst := New(runner)

			as.Equal(defaultOptions, inst.opts)
		},
		"later options override previous": func(t *testing.T) {
			as := newAssertions(t)

			expectedOpts := &options{
				constrained: constraintOptions{
					timeout: 3 * time.Second,
				},
			}

			var runner Runnable
			opts := []Option{
				Timeout(1 * time.Second),
				Timeout(3 * time.Second),
			}
			inst := New(runner, opts...)

			as.Equal(expectedOpts, inst.opts)
		},
	}

	for name, test := range subtests {
		t.Run(name, test)
	}
}
