# PLAN: JSON/JSONC Formatter Refactor — Drop tidwall/pretty, hujson-only

## Status: IN PROGRESS — REGRESSION DETECTED

**Branch:** feat/3.0  
**Stash:** `git stash pop` to resume  
**Parity BEFORE refactor:** 79.2% (346/437)  
**Parity AFTER refactor (broken):** 66.1% (289/437) — JSON dropped from 86% to 42%  
**All unit tests pass.** The regression is in real-world files, not test fixtures.

## What Was Done

1. ✅ Removed tidwall/pretty from `jsonfmt/json.go`
2. ✅ Added `FormatValue` export to `jsoncfmt/jsonc.go`
3. ✅ JSON formatter now delegates to JSONC format engine with `TrailingCommas=none`
4. ✅ Added `isInlineObject` — collapses objects that fit on one line
5. ✅ Added `inlineValueLength` — recursive length calculation
6. ✅ Added `formatInlineValue` — recursive whitespace normalization for collapsed values
7. ✅ Added `parentHasBlankLines` flag — prevents child object collapsing when parent has blank lines
8. ✅ Updated `isInlineArray` to use `inlineValueLength` (allows nested objects in arrays)
9. ✅ Added `MaxLineWidth: 80` to JSONC DefaultOptions
10. ✅ Removed `tidwall/pretty` from go.mod via `go mod tidy`
11. ✅ All unit tests updated and passing (22 packages green)
12. ✅ golangci-lint: 0 issues

## The Problem

Parity REGRESSED from 79% to 66%. JSON went from 86% match to 42%. The object collapsing is doing something wrong on real-world files that the unit tests don't catch.

**Root cause hypothesis:** The collapsing is likely TOO aggressive — collapsing objects that prettier does NOT collapse. Possible issues:

1. **Line-width calculation is wrong.** `isInlineObject` uses `depth * len(indent)` as prefix length but doesn't account for the key name. A key like `"dependencies": { ... }` adds 17 chars to the line that aren't counted.

2. **Prettier has additional rules we haven't captured.** For example, prettier might:
   - Never collapse objects with more than N members
   - Never collapse when a value is itself a nested object (even if it fits)
   - Use the FULL line length (key + colon + value) not just the value length

3. **The JSONC engine's `formatObject` is being used for JSON but has JSONC-specific behaviors** (trailing comma handling, comment preservation) that produce different whitespace than prettier for plain JSON.

## What To Do Tomorrow

### Step 1: Diagnose (15 min max)

Pop the stash. Pick ONE real-world JSON file that was matching before (79% run) but fails now (66% run). Compare:
- What cfv produces now (hujson engine)
- What prettier produces
- What cfv produced BEFORE (snapshot in `/tmp/parity-before/`)

This will immediately show what the collapsing is doing wrong.

### Step 2: Fix the root cause (not symptoms)

Based on what Step 1 reveals, fix the collapsing logic. Likely fixes:
- Add key name length to the prefix calculation in `isInlineObject`/`isInlineArray`
- Or: match prettier's exact algorithm (which they document in their source)

### Step 3: Verify with snapshot

Compare ALL 437 files: new cfv output vs before-snapshot. The ONLY acceptable diffs are:
- Objects that SHOULD collapse (fits under 80) now collapse → improvement
- Everything else must be identical to before → no regressions

### Step 4: Parity check

Run full parity suite. Must be ≥ 79.2% (no regression) and ideally higher (object collapsing gains).

## Key Files

| File | Role |
|------|------|
| `pkg/formatter/jsoncfmt/jsonc.go` | The format engine (shared by JSON + JSONC) |
| `pkg/formatter/jsonfmt/json.go` | Thin wrapper: validates strict JSON, delegates to jsoncfmt |
| `/tmp/parity-before/` | Snapshot of cfv output BEFORE refactor (3393 JSON files) |
| `~/.cfv-parity/run` | Parity test suite runner |
| `/tmp/taplo` | taplo binary for TOML parity |
| `/tmp/cfv-test` | Built cfv binary |

## Commands to Resume

```bash
cd /Users/se456c/src/github.com/boeing/config-file-validator
git stash pop
go build -o /tmp/cfv-test ./cmd/cfv/

# Pick a failing file and diagnose:
F=~/.cfv-parity/repos/hugo/internal/warpc/js/package.json
cp "$F" /tmp/diag.json
/tmp/cfv-test format --fix --no-config --no-editorconfig /tmp/diag.json
diff /tmp/diag.json <(npx prettier --parser json --no-editorconfig "$F")

# Compare against pre-refactor snapshot:
REL=$(echo "$F" | sed "s|$HOME/.cfv-parity/repos/||")
diff /tmp/diag.json "/tmp/parity-before/$REL"
```

## Architecture (correct, keep this)

```
JSON:  json.Valid() → hujson.Parse() → sortKeys → formatWalk → Pack()
JSONC: hujson.Parse() → sortKeys → formatWalk → Pack()

formatWalk (one pass):
  formatValue → formatObject or formatArray
    → isInlineObject/isInlineArray (fit check)
    → if fits: formatInlineValue (recursive single-line normalization)
    → if not: expand with indentation, preserve blank lines, recurse
```

## Non-Negotiables

- ONE library (hujson), ONE parse, ONE walk, ONE serialize
- No tidwall/pretty
- No double scanning
- Parity must not regress below 79.2%
- All unit tests must pass
- golangci-lint clean
