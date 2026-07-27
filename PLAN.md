# PLAN.md

This is the single source of truth for in-progress and planned work. A new session must read this file before doing anything else.

---

## Active Plan A: Sync main → feat/3.0

**Status:** Not started  
**Branch:** `feat/3.0` (merge/cherry-pick from `main`)

### Context

Several commits have landed on `main` that are not yet in `feat/3.0`. They fall into three categories:

1. **Substantive fixes** — must be in feat/3.0 before the v3 release
2. **CI/infra** — already synced via PR #610 (`chore/sync-megalinter-osv-fixes`)
3. **main-only features** — need evaluation before bringing over

### Commits to merge (confirmed via `git log origin/feat/3.0..origin/main`)

| Commit | Title | Action |
|---|---|---|
| `22b6c12` | fix(windows): encode local schema paths as file URLs (#553) | Cherry-pick — bug fix, applies to v3 |
| `a7332f0` | Fix/546 xml optional DTD validation (#548) | Cherry-pick — bug fix, applies to v3 |
| `f1ce95a` | fix(deps): resolve 5 npm vulnerabilities in website/package-lock.json (#579) | Cherry-pick — already superseded by our #610 bump; check if feat/3.0 has newer versions already |
| `bd3b45e` | docs: update stale functional testing section (#555) | Cherry-pick — docs update |
| `33c6241` | feat(cli): add watch mode (#510) | Evaluate — check if feat/3.0 already has watch mode or a conflict |
| `a402f3c` | docs: dynamic format list (#600) | Skip — targets `main` docs site; feat/3.0 has its own docs state |
| `3fe9115` | chore: update github-linguist SHA (#609) | Cherry-pick — linguist update applies to both |
| Dependabot bumps (`573`–`576`) | actions/setup-go, setup-node, codeql, AUR deploy | Cherry-pick all — CI config, no conflicts expected |

### Approach

```
git checkout feat/3.0
git cherry-pick 22b6c12 a7332f0 bd3b45e 3fe9115
# npm vulns — check feat/3.0 package-lock.json first, may already be newer
# watch mode — inspect 33c6241 for conflicts with feat/3.0 cfv.go
# dependabot bumps — cherry-pick .github/workflows changes only
```

### Test strategy

After cherry-picks:
```
go vet ./...
go build -o /dev/null cmd/cfv/cfv.go
go test ./...
```

Resolve any conflicts before moving to Plan B.

---

## Active Plan B: Coverage to ≥ 90% on all packages

**Status:** Not started  
**Branch:** `feat/3.0` (new commits on top)  
**Requirement:** Every package must be ≥ 90%. Currently total is 91.0% but 8 packages are below floor.

### Current coverage gaps (measured on feat/3.0, 2026-07-27)

| Package | Current | Gap | Priority |
|---|---|---|---|
| `pkg/formatter/hclfmt` | 66.7% | -23.3pp | P1 — worst offender |
| `pkg/fixer` | 77.2% | -12.8pp | P1 |
| `pkg/formatter/yamlfmt` | 83.8% | -6.2pp | P2 |
| `pkg/formatter/jsoncfmt` | 87.1% | -2.9pp | P2 |
| `pkg/formatter/xmlfmt` | 87.7% | -2.3pp | P2 |
| `pkg/cli` | 87.5% | -2.5pp | P2 |
| `pkg/formatter/jsonfmt` | 89.2% | -0.8pp | P3 |
| `pkg/configfile` | 89.5% | -0.5pp | P3 |

---

### Task B1: `pkg/formatter/hclfmt` — 66.7% → ≥ 90%

**File:** `pkg/formatter/hclfmt/hcl.go`  
**Uncovered:** `Format` at 66.7% — only 2/3 branches covered.

**What to investigate:** `hcl.go:Format` has an error path (hclwrite fails to parse) and likely an early-return on empty input. Read the function, identify the uncovered branches, add test cases.

**Tests to add** in `pkg/formatter/hclfmt/hcl_test.go`:
- Invalid HCL input returns error
- Empty input behavior
- Tab indent option (if not already covered)
- CRLF line ending option

**Test strategy:**
```
go test -cover -coverprofile /tmp/hcl.out ./pkg/formatter/hclfmt/...
go tool cover -func /tmp/hcl.out | grep -v 100
```
Target: ≥ 90%.

---

### Task B2: `pkg/fixer` — 77.2% → ≥ 90%

**Files:** `fixer.go`, `json_schema_walker.go`, `json_trailing_comma.go`, `schema_string_to_bool.go`, `schema_string_to_int.go`

**Uncovered functions (0.0%):**
- `fixer.go:WithUnsafe` — option setter never exercised
- `json_schema_walker.go:walkArray` — array schema walking never triggered
- `json_trailing_comma.go:ID` — trivial method, never called in tests
- `schema_string_to_bool.go:ID` — same
- `schema_string_to_int.go:ID` — same

**Partially covered:**
- `fixer.go:sortFixes` (66.7%) — sorting with equal-priority fixes not tested
- `fixer.go:fixLess` (66.7%) — same path
- `fixer.go:applyFixes` (85.7%) — overlapping fix byte range not tested
- `json_schema_walker.go:readStringContent` (47.8%) — escape sequences not fully exercised
- `json_schema_walker.go:skipString` (77.8%) — unterminated string path
- `json_trailing_comma.go:skipJSONString` (66.7%) — escape handling
- `schema_string_to_bool.go:Detect` (86.4%) — edge cases in bool string detection
- `schema_string_to_int.go:Detect` (84.2%) — edge cases in int string detection

**Tests to add** in `pkg/fixer/fixer_test.go`:
- `WithUnsafe` option: verify unsafe fixes run when enabled, skipped when not
- `walkArray`: schema with array property containing typed items → string-to-int fix inside array
- `ID()` methods: trivially call `rule.ID()` on each fixer type
- `sortFixes`: two fixes at same offset, same length — verify stable sort
- `applyFixes`: overlapping fixes — verify the later one is dropped, not applied
- `readStringContent`: escape sequences `\"`, `\\`, `\n`, `\uXXXX`
- `skipString`/`skipJSONString`: unterminated string (EOF mid-string)

**Test strategy:**
```
go test -cover -coverprofile /tmp/fixer.out ./pkg/fixer/...
go tool cover -func /tmp/fixer.out | grep -v 100
```
Target: ≥ 90%.

---

### Task B3: `pkg/formatter/yamlfmt` — 83.8% → ≥ 90%

**Uncovered / low coverage functions:**

- `printer.go:shiftBlockScalarIndent` (0.0%) — never called. Verify whether this is dead code or a missing test case. If dead code, remove it. If not, write a test.
- `printer.go:buildStructuralLineSet` (75.0%) — some structural line types not exercised
- `printer.go:buildASTMetadata` (75.0%) — error path (unmarshal fails)
- `printer.go:writeFlowScalar` (68.4%) — anchor, null, and style variants not all covered
- `printer.go:escapeDoubleQuoted` (50.0%) — only some escape sequences tested
- `printer.go:needsQuotingInFlow` (63.6%) — not all characters/types triggering quoting tested
- `printer.go:looksNumeric` (66.7%), `flowHasComments` (66.7%) — partial coverage
- `tokenizer.go:isFullyQuotedSingle` (57.1%), `isFullyQuotedDouble` (60.0%) — edge cases
- `tokenizer.go:detectBlockContentIndent` (53.8%) — low coverage on block scalar indent detection
- `tokenizer.go:lastIndentLevel` (66.7%)

**Note:** `yamlfmt` also has the pre-existing fuzz failure (`-  \r` idempotency bug). That is separate from coverage and is tracked in decisions.md. Do NOT fix it as part of this task — it requires its own plan entry.

**Tests to add** in `pkg/formatter/yamlfmt/yaml_test.go` and `printer_test.go`:
- Flow scalar with anchor: `{&anchor key: value}`
- Flow scalar null: `{key: ~}`  
- Double-quoted scalar with all escape types: `\n`, `\t`, `\r`, `\\`, `\"`
- `isFullyQuotedSingle` edge cases: empty string, single char, unterminated
- Block scalar with explicit indent indicator: `|2-`
- `shiftBlockScalarIndent`: determine if reachable; if not, delete the function

**Test strategy:**
```
go test -cover -coverprofile /tmp/yaml.out ./pkg/formatter/yamlfmt/...
go tool cover -func /tmp/yaml.out | grep -v 100
```
Target: ≥ 90%. If `shiftBlockScalarIndent` is unreachable dead code, deleting it counts as improvement.

---

### Task B4: `pkg/formatter/jsoncfmt` — 87.1% → ≥ 90%

**Uncovered / low coverage:**
- `jsonc.go:clearWhitespace` (50.0%) — the `hasComment` branch not exercised
- `jsonc.go:ensureTrailingComma` (66.7%) — non-nil extra path
- `jsonc.go:sortObject` (77.8%) — array branch not exercised during sort
- `jsonc.go:formatArray` (78.1%) — empty array with comment path
- `jsonc.go:buildIndent` (83.3%) — tabs path
- `jsonc.go:hasTrailingComma` (75.0%) — array branch

**Tests to add** in `pkg/formatter/jsoncfmt/jsonc_test.go`:
- `clearWhitespace` with inline comment: `{"key": 1 /* comment */}` → verify comment preserved, whitespace stripped
- `ensureTrailingComma` with non-nil extra already present
- `sortObject` on array containing objects: `{"arr": [{"z": 1}, {"a": 2}]}`
- Empty array with comment: `{"key": [/* comment */]}`
- Tab indent: `opts.IndentStyle = IndentTabs`
- `hasTrailingComma` on array: `{"arr": [1, 2,]}`

**Test strategy:**
```
go test -cover -coverprofile /tmp/jsonc.out ./pkg/formatter/jsoncfmt/...
go tool cover -func /tmp/jsonc.out | grep -v 100
```
Target: ≥ 90%.

---

### Task B5: `pkg/formatter/xmlfmt` — 87.7% → ≥ 90%

**Uncovered / low coverage:**
- `printer.go:stripTrailingWhitespace` (56.0%) — most paths not hit
- `printer.go:applySelfClosingSpace` (70.0%) — space option variants
- `printer.go:prevIsTag` (66.7%) — false branch
- `printer.go:needsNewlineBefore` (75.0%) — some token type combos
- `printer.go:buildIndentString` (83.3%) — tabs path

**Tests to add** in `pkg/formatter/xmlfmt/xml_test.go`:
- Self-closing element with `SelfClosingSpace=false` option
- Content with trailing whitespace on lines
- Mixed content (text and elements siblings) — exercises `prevIsTag` false path
- Tab indent option

**Test strategy:**
```
go test -cover -coverprofile /tmp/xml.out ./pkg/formatter/xmlfmt/...
go tool cover -func /tmp/xml.out | grep -v 100
```
Target: ≥ 90%.

---

### Task B6: `pkg/cli` — 87.5% → ≥ 90%

**Uncovered functions (0.0%):**
- `cli.go:defaultFixRules` — the fix rules factory, never called in tests
- `cli.go:attemptFix` — the fix pipeline entry point, never called in tests
- `cli.go:resolveSchemaBytes` — schema byte resolution, never called
- `cli.go:Remove` (format.go) — cleanup function, never called

**Partially covered:**
- `cli.go:runSingle` (83.3%) — some error paths
- `cli.go:validateWithExternal` (82.4%) — schema fetch failure path
- `cli.go:toSchemaURL` (83.3%) — error path
- `cli.go:isBrokenSymlink` (85.7%) — non-symlink path

**Note:** `defaultFixRules`, `attemptFix`, and `resolveSchemaBytes` are the `cfv check --fix` pipeline. These are integration-level concerns. The txtar test suite in `cmd/cfv/testdata/` is the right place to cover them end-to-end. Adding txtar tests for `cfv check --fix` covers these functions without requiring complex unit test mocking.

**Tests to add:**
- Txtar: `cmd/cfv/testdata/check_fix_basic.txtar` — run `cfv check --fix` on a file with a trailing comma, verify it's fixed
- Txtar: `cfv check --fix` on a file that already passes — verify no change
- Unit: `isBrokenSymlink` with a regular file (non-symlink path)
- Unit: `toSchemaURL` with malformed path

**Test strategy:**
```
go test -cover -coverprofile /tmp/cli.out ./pkg/cli/...
go tool cover -func /tmp/cli.out | grep -v 100
```
Target: ≥ 90%.

---

### Task B7: `pkg/formatter/jsonfmt` — 89.2% → ≥ 90% (already in Plan A)

Covered by the existing PLAN.md tasks carried over. Two tests needed:
- `TestPreserveBlankLinePrefixNoBlankLine` (covers `preserveBlankLinePrefix` early return)
- `TestTabIndentCollapsesShortArrays` (covers `indentString` tabs path)

---

### Task B8: `pkg/configfile` — 89.5% → ≥ 90%

**Uncovered:** `configfile.go:Load` (85.7%) and `Discover` (90.9%) have minor error paths.

**Tests to add:**
- `Load` with a TOML file that has a key unknown to the schema → verify `additionalProperties` rejection
- `Discover` walking up to a directory that has no `.cfv.toml` → returns nil without error

**Test strategy:**
```
go test -cover -coverprofile /tmp/cfg.out ./pkg/configfile/...
go tool cover -func /tmp/cfg.out | grep -v 100
```
Target: ≥ 90%.

---

## Known Bug: yamlfmt `addFlowMappingPadding` — doubled single-quote closing brace

**Status:** Not started  
**File:** `pkg/formatter/yamlfmt/printer.go`  
**Discovered:** PR #596 review  

### Symptom

Flow mapping values with single-quoted strings containing `''` escaped quotes don't get the closing `}` space:

```
Input:  {key: 'it''s a test'}
Output: { key: 'it''s a test'}   ← missing space before }
Expect: { key: 'it''s a test' }
```

### Root cause

In `addFlowMappingPadding`, the doubled-quote detection:

```go
if quote == '\'' && i+1 < len(raw) && raw[i+1] == '\'' {
    continue  // ← skips escaped = false reset
}
```

The `continue` skips `escaped = false`. Needs focused debug — write the failing test first, then trace through.

### Fix approach

Write a targeted test first:
```go
{"doubled_single_quote", "x: {key: 'it''s a test'}\n", "x: { key: 'it''s a test' }\n"},
```

In `TestNormalizeFlowCollections` in `pkg/formatter/yamlfmt/yaml_test.go`.

---

## Known Bug: jsoncfmt indent-aware collapse (`isInlineArray` depth blindness)

**Status:** Not started  
**File:** `pkg/formatter/jsoncfmt/jsonc.go`  
**Discovered:** PR #569 review  

### Problem

`isInlineArray()` measures only content width, not full line width (indent + key + content). Arrays at deep nesting collapse to lines >80 chars.

**Confirmed:** `dependsOn` array at indent level 3 produces a 95-char line.

### Fix

Change `isInlineArray` to accept `depth int` and `indentLen int`:
```go
func isInlineArray(arr *hujson.Array, depth, indentLen, printWidth int) bool {
    contentLen := 2
    for _, el := range arr.Elements { ... }
    lineLen := depth*indentLen + contentLen
    return lineLen < printWidth
}
```
Update call site: `isInlineArray(arr, depth, len(fs.indent), 80)`

---

## Completion checklist (Plan A + B)

### Plan A — Sync
- [ ] Cherry-pick `22b6c12` (Windows schema paths)
- [ ] Cherry-pick `a7332f0` (XML DTD)
- [ ] Cherry-pick `bd3b45e` (docs)
- [ ] Cherry-pick `3fe9115` (linguist)
- [ ] Cherry-pick dependabot action bumps (#573–#576)
- [ ] Evaluate `33c6241` (watch mode) for conflicts
- [ ] Verify npm package-lock.json — confirm feat/3.0 already has newer versions than `f1ce95a`
- [ ] `go vet ./...` clean
- [ ] `go build -o /dev/null cmd/cfv/cfv.go` clean
- [ ] `go test ./...` passes

### Plan B — Coverage
- [ ] B1: hclfmt ≥ 90%
- [ ] B2: fixer ≥ 90%
- [ ] B3: yamlfmt ≥ 90% (excl. pre-existing fuzz bug)
- [ ] B4: jsoncfmt ≥ 90%
- [ ] B5: xmlfmt ≥ 90%
- [ ] B6: pkg/cli ≥ 90%
- [ ] B7: jsonfmt ≥ 90%
- [ ] B8: configfile ≥ 90%
- [ ] Full pipeline: `go test -cover ./...` total ≥ 90%, all packages ≥ 90%
- [ ] `golangci-lint run ./...` — 0 issues
- [ ] CHANGELOG.md updated under `[Unreleased]` → `Changed`/`Fixed` as appropriate
- [ ] Commit: `test: bring all packages to ≥ 90% coverage`

---

## Bug Fix Plan: yamlfmt FuzzYAMLFormatter idempotency failure on bare `\r`

**Status:** Not started  
**File:** `pkg/formatter/yamlfmt/printer.go`  
**Fuzz corpus:** `pkg/formatter/yamlfmt/testdata/fuzz/FuzzYAMLFormatter/a2e78bc602b45095`  
**Reproducer:** `Format(Format("-  \r")) != Format("-  \r")`

---

### Root cause (fully traced)

Input: `"-  \r"` (sequence dash, two spaces, bare carriage return — no LF)

**Pass 1 tokenization:**
1. `consumeLineContent` sees `-` followed by ` ` → emits `TokDash("- ")`, pos advances to 2
2. pos=2 is second space; `consumeRestOfLine` finds no colon, scans until `\r` → emits `TokValue(" ")`
3. `consumeLineContent` sees `\r` → emits `TokNewline("\r")`

Token stream: `[TokDash("- "), TokValue(" "), TokNewline("\r")]`

**Pass 1 serialization (`serializeWithStrip`):**

The byte loop in `serializeWithStrip` only flushes (with trailing-space stripping) on `\n`:

```go
for _, b := range tok.Raw {
    if b == '\n' {
        // ... flush with stripping
    } else {
        line = append(line, b)  // ← \r goes here, not stripped
    }
}
```

- `TokDash("- ")` → line = `"- "`
- `TokValue(" ")` → line = `"-  "`
- `TokNewline("\r")` → the `\r` byte hits the `else` branch → line = `"-  \r"`
- End of tokens → `flushLineStripped()` trims spaces/tabs but NOT `\r` → out = `"-  \r"`

**Pass 1 post-processing:**

`NormalizeLineEndings` converts `\r` to `\n` → final output: `"-  \n"`

The trailing spaces before the `\r` were never stripped because the flush only fires on `\n`.

**Pass 2:**

Input is now `"-  \n"`. The `\n` triggers `flushLineStripped()` → spaces are stripped → output is `"-\n"`.

`Format(pass1) != pass1` — not idempotent.

---

### Fix

**Location:** `serializeWithStrip` in `pkg/formatter/yamlfmt/printer.go`, the byte-loop inner section.

**What to change:** The `\r` byte must also trigger a line flush with stripping — but only when it's a standalone CR (not followed by LF, which is the CRLF case already handled). The existing code already handles CRLF correctly (it peeks at the next byte when it sees `\n`). The fix is symmetric: treat a standalone `\r` the same as `\n` in the flush logic.

**Current code (simplified):**
```go
for _, b := range tok.Raw {
    if b == '\n' {
        hasCR := len(line) > 0 && line[len(line)-1] == '\r'
        if hasCR {
            line = line[:len(line)-1]
        }
        flushLineStripped()
        if hasCR {
            out = append(out, '\r')
        }
        out = append(out, '\n')
    } else {
        line = append(line, b)
    }
}
```

**Problem:** The loop uses `for _, b := range tok.Raw` which iterates byte-by-byte but cannot look ahead (range over []byte gives index, value pairs — we can use index-based loop instead).

**Fix:** Switch to index-based loop so we can peek at the next byte, then handle bare `\r` as a line terminator:

```go
raw := tok.Raw
for i := 0; i < len(raw); i++ {
    b := raw[i]
    if b == '\n' {
        // Existing CRLF handling (check if line ends with \r).
        hasCR := len(line) > 0 && line[len(line)-1] == '\r'
        if hasCR {
            line = line[:len(line)-1]
        }
        flushLineStripped()
        if hasCR {
            out = append(out, '\r')
        }
        out = append(out, '\n')
    } else if b == '\r' {
        // Bare CR (not followed by LF): treat as a line terminator.
        // This matches how \n is handled — flush with stripping, emit \r.
        // Without this, trailing spaces before a bare \r survive pass 1 but
        // are stripped on pass 2 (after NormalizeLineEndings converts \r→\n),
        // causing idempotency to break.
        if i+1 < len(raw) && raw[i+1] == '\n' {
            // This is CRLF — the \n handler will deal with it on the next
            // iteration. Just accumulate the \r into line so the \n handler
            // can detect and emit it.
            line = append(line, b)
        } else {
            // Standalone \r: flush now.
            flushLineStripped()
            out = append(out, '\r')
        }
    } else {
        line = append(line, b)
    }
}
```

**Why this is correct:**
- Standalone `\r` (old Mac line ending): flush + strip trailing spaces + emit `\r`. NormalizeLineEndings will convert to LF if needed. Now idempotent.
- `\r\n` (CRLF): `\r` goes into line buffer, then `\n` triggers the existing CRLF path (peeks back at last byte of line, removes `\r`, flushes, emits `\r\n`). Unchanged behavior.
- `\n` only: unchanged.

**Alternative simpler fix:**

Since `NormalizeLineEndings` always runs after `serializeWithStrip`, we could normalize `\r` to `\n` BEFORE calling `serializeWithStrip` instead of after. This would mean bare `\r` becomes `\n` before the strip loop, so the `\n` handler strips the trailing spaces correctly on pass 1.

```go
// In printFormatted, before calling serializeWithStrip:
// Normalize \r\n and bare \r to \n in token raw bytes so serializeWithStrip
// only has to handle \n as a line terminator.
normalizeTokenLineEndings(tokens)
out := serializeWithStrip(tokens)
// NormalizeLineEndings still runs after to apply the target line ending.
```

This is simpler but requires a token-normalizing pass. The index-loop fix is more surgical and doesn't add a pass.

**Chosen approach:** Index-loop fix in `serializeWithStrip`. Simpler change, precisely targeted.

---

### Test strategy

**Regression test** (add to `TestReindentIdempotent` or as its own test):
```go
func TestIdempotencyBareCarriageReturn(t *testing.T) {
    inputs := []string{
        "-  \r",          // bare CR after spaces
        "key: value  \r", // bare CR after value with spaces
        "-  \r\n",        // CRLF must still work correctly
        "key: v\r\nother: w\r\n", // full CRLF document unchanged
    }
    opts := yamlfmt.DefaultOptions()
    for _, input := range inputs {
        first, err := yamlfmt.Formatter{}.Format([]byte(input), opts)
        require.NoError(t, err)
        second, err := yamlfmt.Formatter{}.Format(first, opts)
        require.NoError(t, err)
        require.Equal(t, string(first), string(second),
            "not idempotent for input %q", input)
    }
}
```

**Fuzz corpus file** — the existing reproducer `testdata/fuzz/FuzzYAMLFormatter/a2e78bc602b45095` must pass after the fix.

**Semantic check** — verify that `"-  \r"` formats to `"-\n"` on pass 1 (same as `"-  \n"` which already formats to `"-\n"`). The trailing spaces must be gone on the first pass.

---

### Affected files

- `pkg/formatter/yamlfmt/printer.go` — fix `serializeWithStrip` byte loop
- `pkg/formatter/yamlfmt/yaml_test.go` — add `TestIdempotencyBareCarriageReturn`

---

### Pipeline after fix

```
go vet ./pkg/formatter/yamlfmt/...
gofmt -s -l pkg/formatter/yamlfmt/
golangci-lint run ./pkg/formatter/yamlfmt/...
go test -count=1 ./pkg/formatter/yamlfmt/...
# Confirm fuzz corpus file passes:
go test -run FuzzYAMLFormatter/a2e78bc602b45095 ./pkg/formatter/yamlfmt/...
go tool cover -func coverage.out | grep yamlfmt
```

Coverage should improve — the `\r` branch in `serializeWithStrip` (currently uncovered) will now be exercised by the new test.
