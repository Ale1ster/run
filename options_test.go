package run

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	defaultOptions = &options{
		errChanSize: 0,
		recurring: recurrenceOptions{
			recur:  false,
			period: 0,
		},
		constrained: constraintOptions{
			timeout:  0,
			runLimit: 0,
		},
		restartable: restartOptions{
			restartOnError: false,
			restartLimit:   0,
			resetOnSuccess: false,
			backoff:        nil,
		},
		recoverable: panicOptions{
			calm: false,
		},
	}
)

func apply(t *testing.T, initial *options, opts []Option) *options {
	t.Helper()

	res := initial
	for _, opt := range opts {
		res = opt(res)
	}
	return res
}

func testOptions(t *testing.T) {
	sampleBackoffCounts := []uint64{0, 1, 1, 2, 3, 5, 8}

	cases := []struct {
		name    string
		options []Option
		verify  func(as *assert.Assertions, result *options)
	}{
		{
			name:    "defaults",
			options: []Option{},
			verify: func(as *assert.Assertions, opts *options) {
				as.Equal(defaultOptions, opts)
			},
		},
		{
			name:    "later option overrides preceding",
			options: []Option{Recover(true), Recover(false)},
			verify: func(as *assert.Assertions, opts *options) {
				as.Equal(defaultOptions, opts)
			},
		},
		{
			name:    "WithChanBuffer",
			options: []Option{WithChanBuffer(3)},
			verify: func(as *assert.Assertions, opts *options) {
				expected := &options{
					errChanSize: 3,
				}

				as.Equal(expected, opts)
			},
		},
		{
			name:    "Recur",
			options: []Option{Recur(true)},
			verify: func(as *assert.Assertions, opts *options) {
				expected := &options{
					recurring: recurrenceOptions{
						recur: true,
					},
				}

				as.Equal(expected, opts)
			},
		},
		{
			name:    "Period",
			options: []Option{Period(5 * time.Second)},
			verify: func(as *assert.Assertions, opts *options) {
				expected := &options{
					recurring: recurrenceOptions{
						period: 5 * time.Second,
					},
				}

				as.Equal(expected, opts)
			},
		},
		{
			name:    "Timeout",
			options: []Option{Timeout(3 * time.Second)},
			verify: func(as *assert.Assertions, opts *options) {
				expected := &options{
					constrained: constraintOptions{
						timeout: 3 * time.Second,
					},
				}

				as.Equal(expected, opts)
			},
		},
		{
			name:    "RunLimit",
			options: []Option{RunLimit(42)},
			verify: func(as *assert.Assertions, opts *options) {
				expected := &options{
					constrained: constraintOptions{
						runLimit: 42,
					},
				}

				as.Equal(expected, opts)
			},
		},
		{
			name:    "Restart",
			options: []Option{Restart(true)},
			verify: func(as *assert.Assertions, opts *options) {
				expected := &options{
					restartable: restartOptions{
						restartOnError: true,
					},
				}

				as.Equal(expected, opts)
			},
		},
		{
			name:    "RestartLimit with nil backoff",
			options: []Option{RestartLimit(3, nil)},
			verify: func(as *assert.Assertions, opts *options) {
				// Backup backoff function field,
				// and remove it for equality assertion.
				backoff := opts.restartable.backoff
				opts.restartable.backoff = nil

				expected := &options{
					restartable: restartOptions{
						restartLimit: 3,
					},
				}
				var expectedBackoff time.Duration

				as.Equal(expected, opts)
				// Assert backoff is constant.
				for _, count := range sampleBackoffCounts {
					actual := backoff(count)
					as.Equal(expectedBackoff, actual)
				}
			},
		},
		{
			name: "RestartLimit with custom backoff",
			options: []Option{
				RestartLimit(3, func(c uint64) time.Duration {
					return time.Duration(c) * time.Second
				}),
			},
			verify: func(as *assert.Assertions, opts *options) {
				// Backup backoff function field,
				// and remove it for equality assertion.
				backoff := opts.restartable.backoff
				opts.restartable.backoff = nil

				expected := &options{
					restartable: restartOptions{
						restartLimit: 3,
					},
				}

				as.Equal(expected, opts)
				// Assert backoff is constant.
				for _, count := range sampleBackoffCounts {
					expected := time.Duration(count) * time.Second
					actual := backoff(count)
					as.Equal(expected, actual)
				}
			},
		},
		{
			name:    "ResetOnSuccess",
			options: []Option{ResetOnSuccess(true)},
			verify: func(as *assert.Assertions, opts *options) {
				expected := &options{
					restartable: restartOptions{
						resetOnSuccess: true,
					},
				}

				as.Equal(expected, opts)
			},
		},
		{
			name:    "Recover",
			options: []Option{Recover(true)},
			verify: func(as *assert.Assertions, opts *options) {
				expected := &options{
					recoverable: panicOptions{
						calm: true,
					},
				}

				as.Equal(expected, opts)
			},
		},
		{
			name: "allow panic with default options",
			verify: func(as *assert.Assertions, _ *options) {
				opts := new(options)

				canRecover := opts.calm()

				as.Equal(false, canRecover)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t1 *testing.T) {
			as := newAssertions(t)

			initial := new(options)
			final := apply(t1, initial, tc.options)

			tc.verify(as, final)
		})
	}
}
