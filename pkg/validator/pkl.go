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

// ValidateSyntax attempts to evaluate the provided byte slice as a PKL file.
func (v PklValidator) ValidateSyntax(b []byte) (bool, error) {
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

// ValidateFormat is not yet implemented for PklValidator.
func (v PklValidator) ValidateFormat(b []byte, options any) (bool, error) {
	if options == nil {
		// If no specific format options are provided, consider it valid for now.
		// A more robust implementation would involve Pkl schema validation.
		return true, nil
	}
	// If options are provided, it means a specific format validation is requested,
	// which is not yet implemented.
	return false, ErrMethodUnimplemented
}
