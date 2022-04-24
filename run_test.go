package run

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

var tests = map[string]func(*testing.T){
	"constants": testConstants,
	"panic":     testRunnablePanic,
	"runnable":  testRunnable,
	"options":   testOptions,
	"instance":  testInstance,
	"new":       testNew,
}

func TestRun(t *testing.T) {
	for name, test := range tests {
		t.Run(name, test)
	}
}

// Error type for testing
type TestError struct {
	val interface{}
}

func (te TestError) Error() string {
	return fmt.Sprintf("test error: %v", te.val)
}

func testError(val interface{}) error {
	return TestError{val}
}

func newAssertions(t *testing.T) *assert.Assertions {
	return assert.New(t)
}
