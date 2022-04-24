package run

import (
	"context"
	"fmt"
	"testing"
)

func testRunnable(t *testing.T) {
	subtests := map[string]func(*testing.T){
		"nil runnable panics": func(t *testing.T) {
			as := newAssertions(t)

			as.PanicsWithValue(NilRunnable, func() {
				var r Runnable

				_ = r.run(context.TODO())
			})
		},
		"non-nil runnable runs": func(t *testing.T) {
			as := newAssertions(t)

			errorVal := "runnable error"
			var r Runnable = func(ctx context.Context) error {
				return fmt.Errorf(errorVal)
			}

			err := r.run(context.TODO())

			as.EqualError(err, errorVal)
		},
	}

	for name, test := range subtests {
		t.Run(name, test)
	}
}
