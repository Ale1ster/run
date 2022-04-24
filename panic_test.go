package run

import (
	"fmt"
	"testing"
)

func testConstants(t *testing.T) {
	as := newAssertions(t)

	as.Equal("attempted to run a nil Runnable", NilRunnable)
}

func testRunnablePanic(t *testing.T) {
	subtests := map[string]func(*testing.T){
		"RunnablePanic implements error": func(t *testing.T) {
			as := newAssertions(t)

			p := RunnablePanic{}

			as.Implements((*error)(nil), p)
		},
		"RunnablePanic contains panic value": func(t *testing.T) {
			as := newAssertions(t)

			panicVal := "panic message"
			expected := fmt.Sprintf("runnable panic: %v", panicVal)

			p := RunnablePanic{Value: panicVal}

			as.EqualError(p, expected)
		},
	}

	for name, test := range subtests {
		t.Run(name, test)
	}
}
