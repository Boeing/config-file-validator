package validator

import (
	"context"
	"fmt"

	"github.com/apple/pkl-go/pkl"
)

// PklValidator is used to validate a byte slice that is intended to represent a
// PKL file.
type PklValidator struct {
	evaluatorFactory func(context.Context, ...func(*pkl.EvaluatorOptions)) (pkl.Evaluator, error)
}

// Validate attempts to evaluate the provided byte slice as a PKL file.
func (v PklValidator) Validate(b []byte) (bool, error) {
	ctx := context.Background()

	// Convert the byte slice to a ModuleSource using TextSource
	source := pkl.TextSource(string(b))

	evaluatorFactory := v.evaluatorFactory
	if evaluatorFactory == nil {
		evaluatorFactory = pkl.NewEvaluator
	}

	evaluator, err := evaluatorFactory(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to create evaluator: %w", err)
	}

	_, err = evaluator.EvaluateExpressionRaw(ctx, source, "")
	if err != nil {
		return false, fmt.Errorf("failed to evaluate module: %w", err)
	}

	return true, nil
}
