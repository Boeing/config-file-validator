// Package fixer provides the fix engine for cfv.
//
// A Fixer analyzes source bytes (optionally with a JSON Schema) and produces
// a list of available fixes. Each fix is a targeted byte-range replacement
// that corrects a single issue (syntax error, schema violation, etc.).
//
// Design principles:
//   - Fixes are byte-range edits, not AST rewrites. This keeps the engine
//     format-agnostic and avoids comment/whitespace loss.
//   - Each fix is independently applicable. The engine handles conflicts
//     by applying fixes left-to-right and dropping any that overlap with
//     an already-applied fix.
//   - Safe fixes (--fix) never change semantic meaning beyond correcting
//     the specific violation. Unsafe fixes (--fix --unsafe) may alter
//     meaning (e.g., dropping duplicate keys).
package fixer

// FixCategory classifies what kind of issue a fix addresses.
type FixCategory int

const (
	// FixSyntax corrects a syntax error (e.g., trailing comma in JSON).
	FixSyntax FixCategory = iota
	// FixSchema corrects a schema violation (e.g., string→integer coercion).
	FixSchema
)

// FixSafety indicates whether a fix is safe to apply without human review.
type FixSafety int

const (
	// Safe fixes never change semantic meaning beyond correcting the violation.
	Safe FixSafety = iota
	// Unsafe fixes may alter meaning (e.g., removing duplicate keys keeps last).
	Unsafe
)

// Fix describes a single correctable issue and its replacement.
type Fix struct {
	// RuleID identifies the fix rule (e.g., "json-trailing-comma",
	// "schema-string-to-int").
	RuleID string

	// Message is a human-readable description of what the fix does.
	Message string

	// Category classifies the fix (syntax, schema).
	Category FixCategory

	// Safety indicates whether the fix is safe for automated application.
	Safety FixSafety

	// Line is the 1-based line number where the issue occurs.
	Line int

	// Start is the byte offset of the beginning of the region to replace.
	Start int

	// End is the byte offset of the end of the region to replace (exclusive).
	End int

	// Replacement is the bytes to substitute for src[Start:End].
	Replacement []byte
}

// Result holds the outcome of applying fixes to a file.
type Result struct {
	// Fixed is the corrected source content.
	Fixed []byte

	// Applied is the list of fixes that were successfully applied.
	Applied []Fix

	// Dropped is the list of fixes that were skipped due to conflicts
	// (overlapping byte ranges with a previously applied fix).
	Dropped []Fix
}

// Rule is a single fix rule that can detect and produce fixes for a
// specific class of issue.
type Rule interface {
	// ID returns the unique identifier for this rule (e.g., "json-trailing-comma").
	ID() string

	// Detect analyzes src and returns fixes for issues this rule handles.
	// schema is the raw JSON Schema bytes (nil if no schema available).
	// format is the file format name (e.g., "json", "yaml", "toml").
	Detect(src []byte, schema []byte, format string) []Fix
}

// Fixer applies fix rules to source bytes.
type Fixer struct {
	rules  []Rule
	unsafe bool
}

// New creates a Fixer with the given rules.
func New(rules ...Rule) *Fixer {
	return &Fixer{rules: rules}
}

// WithUnsafe returns a copy of the Fixer that also applies unsafe fixes.
func (f *Fixer) WithUnsafe() *Fixer {
	return &Fixer{rules: f.rules, unsafe: true}
}

// Fix analyzes src and applies all applicable fixes.
// schema is the raw JSON Schema bytes (nil if no schema is available).
// format is the file format name (e.g., "json", "yaml").
//
// Fixes are applied left-to-right by byte offset. If two fixes overlap,
// the earlier one wins and the later one is dropped.
func (f *Fixer) Fix(src []byte, schema []byte, format string) Result {
	// Collect all fixes from all rules.
	var allFixes []Fix
	for _, rule := range f.rules {
		fixes := rule.Detect(src, schema, format)
		for _, fix := range fixes {
			if fix.Safety == Unsafe && !f.unsafe {
				continue // skip unsafe fixes unless opted in
			}
			allFixes = append(allFixes, fix)
		}
	}

	if len(allFixes) == 0 {
		return Result{Fixed: src}
	}

	// Sort fixes by start offset (stable — preserves rule order for same offset).
	sortFixes(allFixes)

	// Apply fixes left-to-right, dropping overlaps.
	return applyFixes(src, allFixes)
}

// sortFixes sorts fixes by Start offset (ascending), then by End (ascending).
func sortFixes(fixes []Fix) {
	for i := 1; i < len(fixes); i++ {
		for j := i; j > 0 && fixLess(fixes[j], fixes[j-1]); j-- {
			fixes[j], fixes[j-1] = fixes[j-1], fixes[j]
		}
	}
}

func fixLess(a, b Fix) bool {
	if a.Start != b.Start {
		return a.Start < b.Start
	}
	return a.End < b.End
}

// applyFixes applies non-overlapping fixes to src left-to-right.
func applyFixes(src []byte, fixes []Fix) Result {
	var applied []Fix
	var dropped []Fix

	// Build output by copying unchanged regions + replacements.
	var out []byte
	cursor := 0

	for _, fix := range fixes {
		if fix.Start < cursor {
			// Overlaps with previously applied fix — drop it.
			dropped = append(dropped, fix)
			continue
		}

		// Copy unchanged region before this fix.
		out = append(out, src[cursor:fix.Start]...)
		// Apply replacement.
		out = append(out, fix.Replacement...)
		cursor = fix.End
		applied = append(applied, fix)
	}

	// Copy remaining bytes after last fix.
	out = append(out, src[cursor:]...)

	return Result{
		Fixed:   out,
		Applied: applied,
		Dropped: dropped,
	}
}
