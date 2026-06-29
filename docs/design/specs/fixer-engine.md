# Fixer Engine Specification

**Status:** Draft  
**Version:** cfv 3.0  
**Date:** 2026-06-29  
**Author:** Design team

## Overview

The fixer engine automatically corrects syntax errors and schema violations in configuration files. It operates on byte-range text edits — not AST reconstruction — making it format-agnostic across all 18 supported file types.

## Design Principles

1. **Byte-range edits over AST rewriting.** cfv supports 18 formats. Building AST-aware fixers for each is unrealistic. Byte-range edits are composable, testable, and format-agnostic.
2. **Single-pass application.** No iterative fix loops. If a fix creates a new issue, the user runs cfv again.
3. **Safe by default.** Only fixes classified as `Safe` are applied unless the user opts in to `Unsafe` fixes.
4. **Fixer ≠ Formatter.** The fixer corrects errors. The formatter normalizes style. They run in sequence, never conflated.

---

## Data Structures

### Edit

A single text replacement at a byte range.

```go
package fixer

// Edit represents a byte-range text replacement.
type Edit struct {
    Start   int    // byte offset in source, inclusive
    End     int    // byte offset in source, exclusive
    NewText string // replacement text (empty = deletion, Start==End = insertion)
}
```

Invariants:
- `Start >= 0`
- `End >= Start`
- `End <= len(src)`
- When `Start == End`, the edit is a pure insertion at that position.
- When `NewText == ""` and `Start < End`, the edit is a pure deletion.

### Fix

A logical fix groups one or more edits that must be applied together.

```go
package fixer

// Fix represents a single fixable issue with its proposed correction.
type Fix struct {
    Edits    []Edit   // one or more edits that constitute this fix (applied atomically)
    Message  string   // human-readable description: "remove trailing comma"
    RuleID   string   // stable machine identifier: "json/trailing-comma"
    Category Category // Syntax, Schema, or Format
    Safety   Safety   // Safe or Unsafe
    Line     int      // 1-based line number for reporting
    Column   int      // 1-based column number for reporting
}

// Category classifies the source of the issue being fixed.
type Category int

const (
    CategorySyntax Category = iota // parser-level syntax error
    CategorySchema                 // schema validation mismatch
    CategoryFormat                 // style/formatting (reserved, routed to Formatter)
)

// Safety classifies whether a fix preserves semantic meaning.
type Safety int

const (
    Safe   Safety = iota // guaranteed to preserve meaning (e.g., remove trailing comma in JSON)
    Unsafe              // may change meaning (e.g., type coercion that loses information)
)
```

A fix is atomic: either all its edits are applied, or none are. Edits within a single fix must not overlap each other.

### FixContext

Contextual information passed from the CLI orchestrator to the fixer.

```go
package fixer

// FixContext carries validation results and metadata to the fixer.
type FixContext struct {
    SchemaErrors []SchemaIssue // structured schema validation errors with byte positions
    FilePath     string        // absolute path to the file being fixed
    FileType     string        // registered file type name (e.g., "json", "yaml", "toml")
}
```

### SchemaIssue

A structured schema validation error with byte-level position information.

```go
package fixer

// SchemaIssue represents a single schema validation error with source position.
type SchemaIssue struct {
    Path      string // JSON pointer to the failing value: "/server/port"
    Message   string // schema error message: "expected integer, got string"
    Keyword   string // JSON Schema keyword that failed: "type", "enum", "required"
    Expected  string // expected type or value: "integer"
    Actual    string // actual type or value: "string"
    Line      int    // 1-based line in source
    Column    int    // 1-based column in source
    ByteStart int    // byte offset of the failing value, inclusive
    ByteEnd   int    // byte offset of the failing value, exclusive
}
```

### Fixer Interface

```go
package fixer

// Fixer produces fixes for a source file given its content and validation context.
type Fixer interface {
    // Fixes analyzes src and returns all applicable fixes.
    // The returned fixes may overlap; the application algorithm resolves conflicts.
    Fixes(src []byte, ctx FixContext) []Fix
}
```

### FixResult

Returned by the application algorithm to report what happened.

```go
package fixer

// FixResult describes the outcome of applying fixes to a file.
type FixResult struct {
    Output       []byte // the fixed file content
    Applied      []Fix  // fixes that were successfully applied
    Dropped      []Fix  // fixes dropped due to overlap conflicts
    Skipped      []Fix  // fixes skipped due to safety filter or exclude-rules
    InvalidAfter bool   // true if the output fails syntax validation (fix produced bad output)
}
```

---

## Fixer Interface — Per-Format Registration

Each file type may optionally provide a `Fixer` implementation. Fixers are registered alongside validators in `pkg/filetype`.

```go
package filetype

// FileType with fixer support.
type FileType struct {
    Name       string
    Extensions []string
    Validator  validator.Validator
    Fixer      fixer.Fixer // nil if no fixer is available for this format
    // ... existing fields
}
```

Formats without a fixer simply skip the fix phase. The CLI reports "no fixer available" for unfixed errors in those formats.

---

## Execution Order

When `cfv --fix .` runs on a single file:

```
┌─────────────────────────────────────────────────────────────┐
│ 1. Read file content (src []byte)                           │
├─────────────────────────────────────────────────────────────┤
│ 2. Validate syntax                                          │
│    → syntaxErr *ValidationError (or nil)                    │
├─────────────────────────────────────────────────────────────┤
│ 3. Validate schema (if schema is available)                 │
│    → schemaErrs []SchemaIssue (with byte positions)         │
├─────────────────────────────────────────────────────────────┤
│ 4. Build FixContext{SchemaErrors, FilePath, FileType}       │
├─────────────────────────────────────────────────────────────┤
│ 5. Call Fixer.Fixes(src, ctx) → []Fix                       │
├─────────────────────────────────────────────────────────────┤
│ 6. Filter fixes by safety level and exclude-rules           │
├─────────────────────────────────────────────────────────────┤
│ 7. Apply(src, fixes) → FixResult                            │
├─────────────────────────────────────────────────────────────┤
│ 8. Post-fix validation: re-validate syntax on output        │
│    → if invalid, discard fix output, report error           │
├─────────────────────────────────────────────────────────────┤
│ 9. Run Formatter on fixed content (if formatter exists)     │
├─────────────────────────────────────────────────────────────┤
│ 10. Write final content to disk                             │
└─────────────────────────────────────────────────────────────┘
```

### Subcommand Semantics

| Command | Fixer runs? | Formatter runs? | Writes to disk? |
|---------|-------------|-----------------|-----------------|
| `cfv check` | No | No | No |
| `cfv check --fix` | Yes | No | Yes |
| `cfv format` | No | Yes (dry-run diff) | No |
| `cfv format --fix` | No | Yes | Yes |
| `cfv --fix` | Yes | Yes | Yes |
| `cfv` (bare) | No | No | No |

---

## Schema Information Flow

```
pkg/cli/cli.go validateSchema()
    │
    │  runs schema validation, gets raw errors
    │
    ▼
*validator.SchemaErrors (current: Errors() []string, Positions)
    │
    │  ENHANCEMENT REQUIRED: add ByteStart, ByteEnd to each error
    │  Validators must map JSON pointer paths back to source byte offsets
    │
    ▼
[]fixer.SchemaIssue (structured, with byte positions)
    │
    │  passed via FixContext
    │
    ▼
Fixer.Fixes(src, ctx)
    │
    │  reads src bytes at SchemaIssue.ByteStart:ByteEnd
    │  decides whether a fix is possible and what safety level it has
    │
    ▼
[]fixer.Fix
```

### Required Enhancements to Existing Code

1. **`*validator.SchemaErrors`** must be extended with byte offsets per error. The `Positions` field currently holds line/column pairs. Add `ByteRanges []ByteRange` where `ByteRange` is `{Start, End int}`.

2. **JSON/YAML/TOML validators** must compute byte offsets for schema error locations. Implementation approach:
   - Parse the source into a positional AST (most Go parsers provide token positions).
   - When a schema error references a JSON pointer path, walk the positional AST to that path and extract the byte range of the value node.

3. **The CLI orchestrator** converts `*validator.SchemaErrors` into `[]fixer.SchemaIssue` before calling the fixer. This is a translation layer, not a fixer responsibility.

---

## Fix Application Algorithm

```go
package fixer

import "sort"

// Apply applies fixes to src and returns the result.
// safetyLevel controls which fixes are applied:
//   - Safe: only Safe fixes
//   - Unsafe: both Safe and Unsafe fixes
func Apply(src []byte, fixes []Fix, safetyLevel Safety, excludeRules map[string]bool) FixResult {
    var applied, dropped, skipped []Fix

    // Step 1: Filter by safety and exclude-rules
    var candidates []Fix
    for _, fix := range fixes {
        if fix.Safety > safetyLevel {
            skipped = append(skipped, fix)
            continue
        }
        if excludeRules[fix.RuleID] {
            skipped = append(skipped, fix)
            continue
        }
        candidates = append(candidates, fix)
    }

    // Step 2: Compute bounding range for each fix
    type bounded struct {
        fix      Fix
        rangeMin int // min Start across all edits
        rangeMax int // max End across all edits
    }
    bounded := make([]bounded, len(candidates))
    for i, fix := range candidates {
        lo, hi := fix.Edits[0].Start, fix.Edits[0].End
        for _, e := range fix.Edits[1:] {
            if e.Start < lo {
                lo = e.Start
            }
            if e.End > hi {
                hi = e.End
            }
        }
        bounded[i] = bounded{fix: fix, rangeMin: lo, rangeMax: hi}
    }

    // Step 3: Sort by rangeMin ascending, then rangeMax ascending (tighter range wins ties)
    sort.SliceStable(bounded, func(i, j int) bool {
        if bounded[i].rangeMin != bounded[j].rangeMin {
            return bounded[i].rangeMin < bounded[j].rangeMin
        }
        return bounded[i].rangeMax < bounded[j].rangeMax
    })

    // Step 4: Greedy non-overlap selection (leftmost wins)
    var accepted []bounded
    lastEnd := 0
    for _, b := range bounded {
        if b.rangeMin >= lastEnd {
            accepted = append(accepted, b)
            lastEnd = b.rangeMax
        } else {
            dropped = append(dropped, b.fix)
        }
    }

    // Step 5: Apply accepted fixes in reverse byte-offset order
    // This preserves correctness: later offsets are unaffected by earlier splices.
    sort.SliceStable(accepted, func(i, j int) bool {
        return accepted[i].rangeMin > accepted[j].rangeMin
    })

    output := make([]byte, len(src))
    copy(output, src)

    for _, b := range accepted {
        // Sort edits within this fix by Start descending
        edits := make([]Edit, len(b.fix.Edits))
        copy(edits, b.fix.Edits)
        sort.SliceStable(edits, func(i, j int) bool {
            return edits[i].Start > edits[j].Start
        })

        for _, edit := range edits {
            before := output[:edit.Start]
            after := output[edit.End:]
            output = make([]byte, 0, len(before)+len(edit.NewText)+len(after))
            output = append(output, before...)
            output = append(output, []byte(edit.NewText)...)
            output = append(output, after...)
        }

        applied = append(applied, b.fix)
    }

    return FixResult{
        Output:  output,
        Applied: applied,
        Dropped: dropped,
        Skipped: skipped,
    }
}
```

### Edge Cases

| Case | Behavior |
|------|----------|
| Zero fixes | Return src unchanged, empty Applied/Dropped/Skipped |
| All fixes overlap | Leftmost one wins, rest dropped |
| Fix with empty Edits slice | Panic (caller bug). Fixer must return at least one edit per fix. |
| Edit.Start == Edit.End, NewText == "" | No-op edit. Harmless, not filtered. |
| Edit.End > len(src) | Panic. Fixer produced invalid offsets. |
| Fix produces invalid syntax | Detected in step 8 (post-fix validation). Output discarded, original preserved, error reported. |
| Multiple edits in one fix overlap each other | Undefined behavior. Fixer implementations must not produce this. Enforced by a debug-mode assertion. |

### Why Single-Pass

| eslint | ruff | biome | cfv (this spec) |
|--------|------|-------|-----------------|
| Up to 10 passes | Few passes | Single pass | Single pass |
| Fixes can create new violations | Fixes can create new violations | Overlap = drop | Overlap = drop |
| Timeout after 10 | Converges fast | Deterministic | Deterministic |

Rationale for cfv:
- Config files are small. Overlapping fixes are rare.
- Syntax fixes (trailing comma, missing bracket) don't create new syntax errors.
- Schema fixes (type coercion) don't cascade — fixing one field doesn't invalidate another.
- If a user has many issues, they run `cfv --fix` twice. Simpler than loop detection and timeout logic.

---

## Rule ID Registry

Rule IDs are stable identifiers. Removing a rule ID is a breaking change. Adding new rules is non-breaking.

### Format: `{format}/{rule-name}`

### Syntax Rules

| Rule ID | Safety | Description | Formats |
|---------|--------|-------------|---------|
| `json/trailing-comma` | Safe | Remove trailing comma before `}` or `]` | JSON |
| `json/single-quotes` | Safe | Replace single quotes with double quotes | JSON |
| `json/unquoted-keys` | Safe | Add double quotes around unquoted object keys | JSON |
| `json/missing-comma` | Safe | Insert comma between adjacent values/pairs | JSON |
| `json/duplicate-key` | Unsafe | Remove second occurrence of duplicate key | JSON |
| `yaml/tab-indent` | Safe | Replace tab indentation with spaces | YAML |
| `yaml/duplicate-key` | Unsafe | Remove second occurrence of duplicate key | YAML |
| `toml/duplicate-key` | Unsafe | Remove second occurrence of duplicate key | TOML |
| `toml/trailing-comma` | Safe | Remove trailing comma in inline tables/arrays | TOML |
| `xml/unclosed-tag` | Safe | Insert closing tag for unclosed element | XML |
| `xml/mismatched-tag` | Unsafe | Rename closing tag to match opening tag | XML |
| `csv/inconsistent-columns` | Unsafe | Pad or truncate row to match header column count | CSV |
| `ini/duplicate-section` | Unsafe | Merge duplicate sections | INI |
| `properties/trailing-backslash` | Safe | Remove trailing backslash on last line of value | Properties |

### Schema Rules

| Rule ID | Safety | Description | Formats |
|---------|--------|-------------|---------|
| `schema/string-to-int` | Safe | Unwrap quoted integer: `"8080"` → `8080` | JSON, YAML, TOML |
| `schema/string-to-float` | Safe | Unwrap quoted float: `"3.14"` → `3.14` | JSON, YAML, TOML |
| `schema/string-to-bool` | Safe | Unwrap quoted boolean: `"true"` → `true` | JSON, YAML, TOML |
| `schema/int-to-string` | Unsafe | Wrap integer in quotes: `8080` → `"8080"` | JSON, YAML, TOML |
| `schema/bool-to-string` | Unsafe | Wrap boolean in quotes: `true` → `"true"` | JSON, YAML, TOML |
| `schema/unwrap-array` | Unsafe | Unwrap single-element array: `[x]` → `x` | JSON, YAML, TOML |
| `schema/wrap-array` | Safe | Wrap scalar in array: `x` → `[x]` | JSON, YAML, TOML |
| `schema/null-to-default` | Unsafe | Replace null with schema default value | JSON, YAML |
| `schema/remove-additional` | Unsafe | Remove property not in schema (`additionalProperties: false`) | JSON, YAML, TOML |

### Schema Rule Logic

The fixer reads the source bytes at `SchemaIssue.ByteStart:ByteEnd` and matches against the schema error:

```
schema error "expected integer, got string" at bytes [45, 51]:
    src[45:51] = `"8080"`
    content inside quotes = "8080"
    is numeric? → yes
    emit: Fix{
        Edits: [{Start: 45, End: 51, NewText: "8080"}],
        RuleID: "schema/string-to-int",
        Safety: Safe,
    }

schema error "expected integer, got string" at bytes [45, 52]:
    src[45:52] = `"hello"`
    content inside quotes = "hello"
    is numeric? → no
    emit: nothing (no fix available)
```

---

## Configuration: exclude-rules

Users can suppress specific fix rules via `.cfv.toml`:

```toml
[fix]
safety = "safe"                     # "safe" (default) or "unsafe"
exclude-rules = [
    "json/duplicate-key",
    "schema/remove-additional",
]
```

### Resolution Order

1. CLI flag `--fix-safety=unsafe` overrides `[fix] safety` in config.
2. CLI flag `--fix-exclude=rule1,rule2` is additive with `[fix] exclude-rules` in config.
3. Env var `CFV_FIX_SAFETY` overrides config, overridden by CLI flag.
4. Env var `CFV_FIX_EXCLUDE` (comma-separated) is additive with all other sources.

Merged exclude set = config ∪ env ∪ CLI flag.

### CLI Flags

```
--fix                Apply fixes and write corrected files to disk
--fix-safety=LEVEL   Safety level: "safe" (default), "unsafe"
--fix-exclude=RULES  Comma-separated rule IDs to exclude from fixing
--fix-dry-run        Show what would be fixed without writing (implies --fix)
```

---

## Post-Fix Validation

After applying fixes, the engine re-validates syntax on the output:

```go
fixed := Apply(src, fixes, safetyLevel, excludeRules)

valid, err := ft.Validator.ValidateSyntax(fixed.Output)
if !valid || err != nil {
    // Fix produced invalid output. Discard and report.
    fixed.InvalidAfter = true
    fixed.Output = src // restore original
    // Report: "fixer produced invalid output; original preserved"
}
```

This is a safety net. A well-implemented fixer should never produce invalid output. If this triggers:
- Log a warning with the rule IDs of applied fixes.
- Include it in the report as an internal error.
- The original file is preserved (no data loss).

Schema is NOT re-validated after fixing. Rationale: schema validation is expensive (network fetch, compilation) and the fixer's schema rules are designed to resolve the specific issues passed in. If a schema fix introduces a new schema error, the user's next run catches it.

---

## Reporter Integration

Fix results are exposed to reporters via an extended report struct:

```go
package reporter

// FileReport is the existing report struct, extended with fix information.
type FileReport struct {
    // ... existing fields (Path, IsValid, Errors, etc.)

    // Fix results (populated only when --fix or --fix-dry-run is active)
    FixesApplied []FixEntry // fixes that were applied
    FixesDropped []FixEntry // fixes dropped due to overlap
    FixesSkipped []FixEntry // fixes skipped due to safety/exclude
    FixInvalid   bool       // true if fix output was discarded (invalid)
}

// FixEntry is a reporter-friendly summary of a fix.
type FixEntry struct {
    RuleID   string // "json/trailing-comma"
    Message  string // "remove trailing comma"
    Line     int    // 1-based
    Column   int    // 1-based
    Category string // "syntax", "schema"
    Safety   string // "safe", "unsafe"
}
```

### Reporter Output Examples

**Stdout (default reporter):**
```
PASS src/config.json
FIXED src/api.json (3 fixes applied, 1 dropped)
  ✓ json/trailing-comma at line 12 — remove trailing comma
  ✓ schema/string-to-int at line 25 — unwrap quoted integer
  ✓ json/trailing-comma at line 31 — remove trailing comma
  ✗ json/single-quotes at line 14 — overlaps with fix at line 12
FAIL src/broken.yaml (unfixable)
```

**JSON reporter:**
```json
{
  "path": "src/api.json",
  "status": "fixed",
  "fixes_applied": [
    {"rule_id": "json/trailing-comma", "message": "remove trailing comma", "line": 12, "column": 45, "safety": "safe"}
  ],
  "fixes_dropped": [
    {"rule_id": "json/single-quotes", "message": "replace single quotes with double quotes", "line": 14, "column": 5, "reason": "overlaps with json/trailing-comma at line 12"}
  ]
}
```

**SARIF reporter:**

Fix information maps to SARIF `fix` objects within `result`:
```json
{
  "results": [{
    "ruleId": "json/trailing-comma",
    "message": {"text": "remove trailing comma"},
    "fixes": [{
      "description": {"text": "remove trailing comma"},
      "artifactChanges": [{
        "artifactLocation": {"uri": "src/api.json"},
        "replacements": [{
          "deletedRegion": {"byteOffset": 156, "byteLength": 1},
          "insertedContent": {"text": ""}
        }]
      }]
    }]
  }]
}
```

### Dry-Run Mode

When `--fix-dry-run` is active:
- All fix computation runs normally.
- `FixResult` is populated.
- No files are written to disk.
- Reporters output what would have been fixed (same format as above, with a "dry-run" indicator).
- Exit code reflects what *would* be the state after fixing (0 if all issues are fixable).

---

## Error Cases Summary

| Scenario | Behavior |
|----------|----------|
| No fixer registered for format | Skip fix phase. Report errors normally. |
| Fixer returns zero fixes | No changes. File reported as unfixed error. |
| Fix produces invalid output | Discard output, preserve original, report internal error. |
| Schema unavailable (network error) | No SchemaIssues in FixContext. Only syntax fixes run. |
| File is read-only | Report error "cannot write fix: permission denied". Original preserved. |
| Edit byte offsets out of bounds | Panic (fixer bug). Caught in tests, not in production gracefully — fixers must be correct. |
| All fixes excluded by rules | No changes. Report: "all fixes excluded by configuration". |

---

## Package Layout

```
pkg/fixer/
├── fixer.go          // Fixer interface, Fix, Edit, Category, Safety types
├── apply.go          // Apply() algorithm
├── result.go         // FixResult type
├── context.go        // FixContext, SchemaIssue types
├── json_fixer.go     // JSON syntax+schema fixer implementation
├── yaml_fixer.go     // YAML syntax+schema fixer implementation
├── toml_fixer.go     // TOML syntax+schema fixer implementation
├── xml_fixer.go      // XML syntax fixer implementation
├── csv_fixer.go      // CSV syntax fixer implementation
├── ini_fixer.go      // INI syntax fixer implementation
├── properties_fixer.go // Properties syntax fixer implementation
├── schema_rules.go   // Shared schema fix logic (type coercion)
├── rules.go          // Rule ID constants and registry
└── fixer_test.go     // Tests (table-driven, per-rule)
```

---

## Implementation Phases

### Phase 1: Core engine (MVP)

- `Edit`, `Fix`, `FixResult`, `FixContext` types
- `Apply()` algorithm with overlap resolution
- `Fixer` interface
- JSON fixer: `json/trailing-comma`, `json/single-quotes`
- Post-fix validation safety net
- `--fix`, `--fix-dry-run` CLI flags
- Stdout reporter integration

### Phase 2: Schema fixes

- Enhance `SchemaErrors` with byte positions
- Byte-offset mapping in JSON/YAML/TOML validators
- Schema fix rules: `schema/string-to-int`, `schema/string-to-float`, `schema/string-to-bool`
- `--fix-safety` flag and `[fix]` config section

### Phase 3: Broad format coverage

- YAML fixer: `yaml/tab-indent`, `yaml/duplicate-key`
- TOML fixer: `toml/trailing-comma`, `toml/duplicate-key`
- XML fixer: `xml/unclosed-tag`, `xml/mismatched-tag`
- Remaining schema rules (`schema/wrap-array`, `schema/unwrap-array`, etc.)
- JSON/SARIF reporter integration

### Phase 4: Unsafe rules and advanced schema

- All `Unsafe` rules
- `schema/null-to-default`, `schema/remove-additional`
- `--fix-exclude` CLI flag
- Full rule ID documentation page

---

## Compatibility and Stability

- Rule IDs are part of the public API. Removing a rule ID is a semver major change.
- Adding new rules is non-breaking (new fixes appear, but `exclude-rules` lets users suppress).
- The `Fixer` interface is public API. Changes require a major version bump.
- Fix output is deterministic: same input + same rules + same config = same output. No randomness, no timestamp-dependent behavior.
- The `Apply()` algorithm's overlap resolution is deterministic: leftmost wins, ties broken by tighter range.
