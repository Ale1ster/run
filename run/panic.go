package run

import "fmt"

const NilRunnable = "attempted to run a nil Runnable"

// RunnablePanic represents a runnable panic in case of successful recovery.
// Its Value field contains the recovered value.
type RunnablePanic struct {
	Value interface{}
}

// Error satisfies error interface for RunnablePanic.
func (p RunnablePanic) Error() string {
	return fmt.Sprintf("runnable panic: %v", p.Value)
}
