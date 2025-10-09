package validator

import (
	"context"
	"errors"
	"fmt"
	"os/exec"


	"github.com/apple/pkl-go/pkl"
)

var (
	// ErrPklSkipped is returned when a validation is skipped due to a missing dependency.
	ErrPklSkipped = errors.New("validation skipped")
)
// PklValidator is used to validate a byte slice that is intended to represent a
// PKL file.
type PklValidator struct {
	evaluatorFactory func(context.Context, ...func(*pkl.EvaluatorOptions)) (pkl.Evaluator, error)
}

// Validate attempts to evaluate the provided byte slice as a PKL file.
// If the 'pkl' binary is not found, it returns ErrSkipped.
func (v PklValidator) Validate(b []byte) (bool, error) {
	ctx := context.Background()

	// Convert the byte slice to a ModuleSource using TextSource
	source := pkl.TextSource(string(b))

	evaluatorFactory := v.evaluatorFactory
	if evaluatorFactory == nil {
		evaluatorFactory = pkl.NewEvaluator
	}

	evaluator, err := evaluatorFactory(ctx, pkl.PreconfiguredOptions)
	if err != nil {
		// If the error is that the pkl binary was not found, return ErrPklSkipped.
		var execErr *exec.Error
		if errors.As(err, &execErr) && execErr.Err == exec.ErrNotFound {
			return false, ErrPklSkipped
		}
		return false, fmt.Errorf("failed to create evaluator: %w", err)
	}

	_, err = evaluator.EvaluateExpressionRaw(ctx, source, "")
	if err != nil {
		return false, fmt.Errorf("failed to evaluate module: %w", err)
	}

	return true, nil
}
