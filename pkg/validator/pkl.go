package validator

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sync"

	"github.com/apple/pkl-go/pkl"
)

var (
	// ErrSkipped is returned when a validation is skipped due to a missing dependency.
	ErrSkipped = errors.New("validation skipped")

	isPklBinaryPresent = func() bool {
		_, err := exec.LookPath("pkl")
		return err == nil
	}
	// mutex for thread-safe modification of the checker function
	mu sync.Mutex
)

// SetPklBinaryChecker allows overriding the default pkl binary check for testing.
// It returns the previous checker function so it can be restored later.
func SetPklBinaryChecker(checker func() bool) func() bool {
	mu.Lock()
	defer mu.Unlock()
	previous := isPklBinaryPresent
	isPklBinaryPresent = checker
	return previous
}

// PklValidator is used to validate a byte slice that is intended to represent a
// PKL file.
type PklValidator struct{}

// Validate attempts to evaluate the provided byte slice as a PKL file.
// If the 'pkl' binary is not found, it returns ErrSkipped.
func (PklValidator) Validate(b []byte) (bool, error) {
	mu.Lock()
	checker := isPklBinaryPresent
	mu.Unlock()

	if !checker() {
		return false, ErrSkipped
	}

	ctx := context.Background()

	// Convert the byte slice to a ModuleSource using TextSource
	source := pkl.TextSource(string(b))

	evaluator, err := pkl.NewEvaluator(ctx, pkl.PreconfiguredOptions)
	if err != nil {
		return false, fmt.Errorf("failed to create evaluator: %w", err)
	}

	_, err = evaluator.EvaluateExpressionRaw(ctx, source, "")
	if err != nil {
		return false, fmt.Errorf("failed to evaluate module: %w", err)
	}

	return true, nil
}
