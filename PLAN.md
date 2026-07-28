# PLAN: Replace tidwall/pretty with hujson-only JSON formatter

## Decision

Drop `tidwall/pretty` from the JSON formatter. Use `hujson` as the single library for both JSON and JSONC formatting. One parse, one format walk, one pack. No double scanning.

## Why

- tidwall/pretty collapses arrays but NOT objects → 78 parity failures
- To work around this, hujson was bolted on top for blank line restoration → double parse
- The JSONC formatter (`jsoncfmt/`) already has a working single-pass hujson format walk
- hujson handles both JSON and JSONC (JSON is a subset)

## Architecture After Refactor

```
JSON input → hujson.Parse → sortKeys → formatWalk → Pack → result
JSONC input → hujson.Parse → sortKeys → formatWalk → Pack → result
```

One parse. One walk. One serialize. Both formats use the same engine.

## What Already Exists (JSONC formatter)

`pkg/formatter/jsoncfmt/jsonc.go` already has:
- `formatState` struct with configurable indent, trailing commas
- `formatObject` — expands objects, applies indentation, handles comments
- `formatArray` — expands arrays OR keeps them inline (`isInlineArray`)
- `isInlineArray` — checks if array fits on one line (primitives only, under 80 cols)
- `sortObject` — sorts object members alphabetically
- `reindentExtra`, `clearWhitespace`, `hasBlankLine`, `addBlankLine` — whitespace helpers
- Blank line preservation (checks original BeforeExtra for blank lines, re-adds them)

## What's Missing (gaps to fill)

1. **Object collapsing**: `formatObject` always expands. Need `isInlineObject` that checks if an object fits on one line (same logic as `isInlineArray` but for objects).

2. **`isInlineArray` is too conservative**: Only allows Literal elements (no nested objects). prettier collapses `[{"type": "home", "number": "123"}]` if it fits. Need to allow nested objects/arrays IF their collapsed form fits.

3. **Key width in line-length calculation**: `isInlineArray` uses `depth * indentLen` as a proxy for the current column. The actual column includes the key name. Need to pass the actual prefix length for accurate measurement.

4. **JSON formatter rewrite**: Replace `jsonfmt/json.go` to use the shared format engine instead of tidwall/pretty.

## Implementation Plan

### Step 1: Extract shared format engine

Create `pkg/formatter/jsonfmt/internal` or better: refactor `jsoncfmt`'s format logic into a shared package that both JSON and JSONC use.

Actually simpler: **merge the JSON formatter into jsoncfmt**. The JSON formatter becomes a thin wrapper:

```go
// pkg/formatter/jsonfmt/json.go
func (Formatter) Format(src []byte, opts formatter.Options) ([]byte, error) {
    if !json.Valid(src) {
        return nil, errors.New("json: invalid JSON input")
    }
    // Use the JSONC engine (JSON is valid JSONC).
    // Override trailing commas to "none" (JSON forbids them).
    opts.TrailingCommas = formatter.TrailingCommasNone
    return jsoncfmt.Formatter{}.Format(src, opts)
}
```

Wait — this won't work because jsoncfmt accepts JSONC (comments/trailing commas in input) which json.Valid() rejects. The JSON formatter should validate input as strict JSON first, THEN delegate to the shared engine.

Better approach: extract the format walk into a shared internal function.

### Step 2: Actual file changes

**File: `pkg/formatter/jsonfmt/json.go`** — REWRITE

```go
package jsonfmt

import (
    "encoding/json"
    "errors"

    "github.com/tailscale/hujson"
    "github.com/Boeing/config-file-validator/v3/pkg/formatter"
    "github.com/Boeing/config-file-validator/v3/pkg/formatter/jsoncfmt"
)

type Formatter struct{}

func DefaultOptions() formatter.Options {
    return formatter.Options{
        IndentStyle:  formatter.IndentSpaces,
        IndentWidth:  2,
        FinalNewline: true,
        SortKeys:     false,
        MaxLineWidth: 80,
    }
}

func (Formatter) Format(src []byte, opts formatter.Options) ([]byte, error) {
    // Strict JSON validation — reject comments and trailing commas.
    if !json.Valid(src) {
        return nil, errors.New("json: invalid JSON input")
    }

    // Parse with hujson (which handles JSON as a subset).
    v, err := hujson.Parse(src)
    if err != nil {
        return nil, err
    }

    // Delegate to the shared JSONC format engine.
    // JSON never has trailing commas, so force "none" for removal.
    opts.TrailingCommas = formatter.TrailingCommasNone
    return jsoncfmt.FormatValue(v, src, opts)
}
```

**File: `pkg/formatter/jsoncfmt/jsonc.go`** — EXPORT format engine

Add exported `FormatValue` function:

```go
// FormatValue formats a pre-parsed hujson Value with the given options.
// This is the shared format engine used by both JSON and JSONC formatters.
func FormatValue(v *hujson.Value, originalSrc []byte, opts formatter.Options) ([]byte, error) {
    resolved := resolveOptions(opts)
    indent := buildIndent(resolved)

    // Sort keys if requested.
    if resolved.SortKeys {
        sortObject(v)
    }

    // Format: one walk of the tree.
    original, _ := hujson.Parse(originalSrc) // for blank line detection
    fs := &formatState{
        indent:              indent,
        maxLineWidth:        resolved.MaxLineWidth,
        trailingCommas:      wantTrailingCommas(v, resolved.TrailingCommas),
        removeTrailingCommas: resolved.TrailingCommas == formatter.TrailingCommasNone,
        original:            &original,
    }
    fs.formatValue(v, 0)

    result := v.Pack()

    // Final newline.
    result = trimTrailingNewlines(result)
    if resolved.FinalNewline {
        result = append(result, '\n')
    }

    return formatter.NormalizeLineEndings(result, resolved.LineEnding), nil
}
```

**File: `pkg/formatter/jsoncfmt/jsonc.go`** — ADD `isInlineObject`

```go
// isInlineObject returns true if the object should be collapsed to one line.
// Objects with only literal values (or inline-able nested collections) that
// fit within maxWidth at the current column are kept inline.
func (fs *formatState) isInlineObject(obj *hujson.Object, depth int, prefixLen int) bool {
    if len(obj.Members) == 0 {
        return true
    }
    // Don't inline if any member has comments or blank lines.
    for i, m := range obj.Members {
        if hasComment(m.Name.BeforeExtra) || hasComment(m.Value.BeforeExtra) ||
           hasComment(m.Name.AfterExtra) || hasComment(m.Value.AfterExtra) {
            return false
        }
        if i > 0 && hasBlankLine(m.Name.BeforeExtra) {
            return false
        }
    }
    // Compute collapsed length: { "key": value, "key2": value2 }
    totalLen := 4 // "{ " + " }"
    for i, m := range obj.Members {
        if i > 0 {
            totalLen += 2 // ", "
        }
        totalLen += len(m.Name.Value.(hujson.Literal)) // key with quotes
        totalLen += 2 // ": "
        valLen := inlineValueLength(&m.Value)
        if valLen < 0 {
            return false // nested value can't be inlined
        }
        totalLen += valLen
    }
    lineLen := prefixLen + totalLen
    return lineLen <= fs.maxLineWidth
}

// inlineValueLength returns the single-line length of a value,
// or -1 if it cannot be inlined (contains comments, multi-line strings, etc).
func inlineValueLength(v *hujson.Value) int {
    switch val := v.Value.(type) {
    case hujson.Literal:
        return len(val)
    case *hujson.Object:
        if len(val.Members) == 0 {
            return 2 // {}
        }
        total := 4 // "{ " + " }"
        for i, m := range val.Members {
            if i > 0 {
                total += 2
            }
            total += len(m.Name.Value.(hujson.Literal)) + 2
            inner := inlineValueLength(&m.Value)
            if inner < 0 {
                return -1
            }
            total += inner
        }
        return total
    case *hujson.Array:
        if len(val.Elements) == 0 {
            return 2 // []
        }
        total := 2 // "[" + "]"
        for i, e := range val.Elements {
            if i > 0 {
                total += 2
            }
            inner := inlineValueLength(&e)
            if inner < 0 {
                return -1
            }
            total += inner
        }
        return total
    default:
        return -1
    }
}
```

**Modify `formatObject`** to check `isInlineObject` before expanding:

```go
func (fs *formatState) formatObject(obj *hujson.Object, depth int, prefixLen int) {
    // ... empty check ...

    // Try to collapse to one line.
    if fs.isInlineObject(obj, depth, prefixLen) {
        // Apply inline formatting.
        for i := range obj.Members {
            m := &obj.Members[i]
            if i == 0 {
                m.Name.BeforeExtra = hujson.Extra(" ")
            } else {
                m.Name.BeforeExtra = hujson.Extra(" ")
            }
            m.Name.AfterExtra = nil
            m.Value.BeforeExtra = hujson.Extra(" ")
            m.Value.AfterExtra = nil
            fs.formatValue(&m.Value, depth+1, 0) // recurse for nested inlining
        }
        obj.AfterExtra = hujson.Extra(" ")
        return
    }

    // ... existing expand logic ...
}
```

### Step 3: Remove tidwall/pretty

- Delete the `pretty` import from `jsonfmt/json.go`
- Delete `restoreBlankLines` and all related functions from `jsonfmt/json.go`
- Run `go mod tidy` to remove unused dependency

### Step 4: Update `isInlineArray` to handle nested objects

Change:
```go
if _, ok := el.Value.(hujson.Literal); !ok {
    return false
}
```

To:
```go
valLen := inlineValueLength(&el)
if valLen < 0 {
    return false
}
totalLen += valLen + 2
```

This allows `[{"type": "home"}]` to be collapsed if it fits.

### Step 5: Pass `prefixLen` through the call chain

`formatObject` and `formatArray` need to know the current column position (not just depth) to make accurate inline decisions. The `prefixLen` is:
- At root: 0
- Under a key `"flagWords": `: depth*indentLen + len(key) + 2 (for `: `)

Add `prefixLen int` parameter to `formatValue`, `formatObject`, `formatArray`.

## Testing Strategy

1. **Existing tests must pass unchanged** — the behavior for JSONC should be identical (it already uses hujson). JSON tests verify the new path produces same output.

2. **New tests for object collapsing**:
   - Short root object → one line
   - Short nested object → one line  
   - Object over 80 chars → expanded
   - Object with nested object that fits → collapsed recursively
   - Object with comments → always expanded

3. **Parity check**: run full suite, expect #569 JSON failures to go to 0.

4. **Idempotency**: format(format(x)) == format(x) for all test files.

## Files Changed

| File | Change |
|------|--------|
| `pkg/formatter/jsonfmt/json.go` | Rewrite: remove tidwall/pretty, delegate to jsoncfmt engine |
| `pkg/formatter/jsoncfmt/jsonc.go` | Export `FormatValue`, add `isInlineObject`, update `isInlineArray`, add `prefixLen` |
| `pkg/formatter/jsonfmt/json_test.go` | Add object collapsing tests |
| `go.mod` / `go.sum` | Remove tidwall/pretty after `go mod tidy` |

## Risk

- tidwall/pretty is also used elsewhere? Check: `grep -r "tidwall/pretty" --include="*.go" .`
- Blank line preservation currently relies on comparing original vs formatted trees. The new approach handles it in one pass (checking original BeforeExtra during the format walk).

## Order of Operations

1. ~~Add `isInlineObject`, `inlineValueLength`, `maxLineWidth` to jsoncfmt~~ ✅ DONE
2. ~~Export `FormatValue` from jsoncfmt~~ ✅ DONE
3. ~~Rewrite jsonfmt to use FormatValue~~ ✅ DONE
4. ~~Remove tidwall/pretty import~~ ✅ DONE (json.go no longer imports it)
5. **FIX: inline formatting must recursively normalize nested values** ← NEXT
   - When an object is collapsed to one line, nested objects need `{ }` padding
   - Nested arrays need `[elem, elem]` spacing (space after comma)
   - Add `applyInlineFormat(v *hujson.Value)` that recursively normalizes for one-line display
6. Update test fixtures to match prettier output (objects now collapse)
7. Remove `json_internal_test.go` ✅ DONE
8. `go mod tidy` to remove tidwall/pretty from go.mod
9. Run full test suite + parity check
10. Code review, stress test

## Current State (stashed as WIP)

- `git stash` contains the refactor in progress
- The architecture is correct: one parse, one walk, one pack
- Object collapsing WORKS (short objects get collapsed)
- `parentHasBlankLines` flag correctly prevents collapsing in blank-line parents
- `formatInlineValue` recursively normalizes nested `{}` and `[]` spacing
- `FormatValue` exported from jsoncfmt, jsonfmt delegates to it
- tidwall/pretty import removed from jsonfmt

## Remaining Work (next session)

### Problem: Tests use short inputs that now collapse

Adding `MaxLineWidth: 80` + object collapsing changes behavior for ALL short inputs.
~20 tests across jsonfmt and jsoncfmt use 2-3 member objects that fit on one line.
Previously these expanded (tidwall/pretty never collapsed objects). Now they collapse (correct prettier behavior).

### Approach: Bulk test update (NOT one-by-one)

1. Pop the stash
2. Run a script that:
   - For each `.input.json` fixture: format with cfv, write to `.expected.json`
   - For each `.input.jsonc` fixture: format with cfv, write to `.expected.jsonc`
   - For fixtures with `.opts.json`: apply those options
3. For standalone tests (TestDefaultTrailingCommas, etc.):
   - Tests that verify EXPANSION behavior: make inputs longer (>80 chars)
   - Tests that verify COLLAPSING behavior: keep short inputs, update expectations
   - Tests that verify TRAILING COMMAS: make inputs longer so they expand
4. Verify each updated fixture matches prettier output
5. Run full suite — should pass
6. Run parity suite — measure improvement

### Key Rule

Every test input that's meant to exercise expanded formatting MUST be >80 chars when collapsed.
Every test input that's meant to exercise collapsed formatting MUST be ≤80 chars when collapsed.
No test should accidentally test the wrong behavior because of input length.
