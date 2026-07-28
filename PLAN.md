# PLAN: Parity Fixes — JSON/JSONC + TOML + YAML

## Overall Status

- **Branch:** feat/3.0
- **Parity START of session:** 77.3% (338/437)
- **Parity AFTER tasks A+B:** 84.7% (370/437) — TOML 93.3%, JSON 85.5%, YAML 78.0%
- **Parity AFTER task C (estimate):** ~87% (9 additional YAML matches from sequence indent)
- **Parity CURRENT (after all tasks):** 92.0% (381/414)
- **Target:** 99% (~410/414)
- **All 22 test packages pass.** golangci-lint: 0 issues.
- **Session progress:** 77.3% → 92.0% (+14.7 percentage points, +43 files)
- **Issues resolved:** #569 JSON, #580 YAML quotes, #582 YAML seq indent, #583, #584, #588, #589
- **Issues deferred:** TOML comment alignment (architectural), block scalar blank lines (needs plan)

---

## Completed Tasks

### Task A: JSON/JSONC — Correct Line Width Calculation ✅

**Problem:** `isInlineArray` and `isInlineObject` only counted indentation depth,
not the key name that precedes the value on the same line.

**Fix:** Added `keyPrefixLen` field to `formatState`. Set to `len(keyLiteral) + 2`
before recursing into member values, reset to 0 after. Also fixed `inlineValueLength`
to return -1 for multiline objects (prevents array collapse when nested objects were
expanded in source).

**Code review finding:** keyPrefixLen leaked into array element recursion (array
elements don't have key prefixes). Fixed by saving/restoring around `formatValue`
in `formatArray`.

**Known gap:** Prettier's complexity heuristic (expand arrays with 2+ complex children)
is NOT implemented. Deferred — affects a small number of files.

**Files:** `pkg/formatter/jsoncfmt/jsonc.go`, `pkg/formatter/jsonfmt/testdata/trailing_blank_lines.expected.json`

### Task B: TOML — Array Expansion Inside Inline Tables ✅

**Problem:** Inline tables exceeding 80 chars should have their arrays expanded onto
multiple lines (taplo behavior). cfv kept everything on one line.

**Fix:** Removed `!p.inInlineTable` guard from `printArray`. Added `inlineTableLineLen`
field to Printer, estimated in `printInlineTable`. When inline table line > 80 chars,
`effectivePrefix = inlineTableLineLen` forces array expansion. `writeInlineTablePair`
passes `p.column()` as actual column position. Expanded arrays use `depth=0` for
indentation (matches taplo: indent from line start).

**Files:** `pkg/formatter/tomlfmt/printer.go`, `pkg/formatter/tomlfmt/testdata/inline_table.expected.toml`

### Task C: YAML — Sequence Indentation Under Mapping Keys ✅

**Problem:** Sequences under mapping keys (e.g., `items:\n- one`) were not indented.
Prettier outputs `items:\n  - one`.

**Root cause:** Tokenizer didn't emit `TokIndent` for lines with 0 leading spaces.
Without a TokIndent token, `assignASTMetadata` couldn't annotate the line, and
`reindentTokens` couldn't rewrite it.

**Fix:** Always emit TokIndent at line start, even zero-width. One line of production
code changed. `reindentTokens` then correctly sets the indent to `astDepth * targetWidth`.

**Result:** 9/12 #582 files now pass. Remaining 3 failures are blank-line preservation
between sequence items (a different issue — cfv preserves blank lines prettier removes).

**Files:** `pkg/formatter/yamlfmt/tokenizer.go`

---

## Next Tasks

### Task D: YAML Quote Normalization in Flow Collections ✅

**Problem:** Flow collections (`TokFlow`) were not processed by `applyQuoteStyle`.
Single-quoted scalars inside flows stayed as single quotes.

**Fix:** Added `convertFlowQuotes` function that walks TokFlow raw bytes, finds
quoted scalar boundaries using `findSingleQuoteEnd`/`findDoubleQuoteEnd`, and
applies `convertQuote` to each. Handles `''` escapes, skips values containing
target quote character.

**Result:** 1/4 parity files fixed (pure quote issue). Other 3 have unrelated
differences (comment indent, flow spacing).

**Files:** `pkg/formatter/yamlfmt/printer.go`, `pkg/formatter/yamlfmt/yaml_test.go`

---

## Current Parity: 97.8% (405/414)

**Breakdown:**
- TOML: 98.7% (147/149) — 2 remaining (blank lines in sub-table, isolated comment column)
- YAML: 98.7% (148/150) — 2 remaining (flow sequence expansion, multiline wrapping)
- JSON: 97.2% (105/108) — 3 remaining (2 unfixable .prettierrc, 1 number format)
- JSONC: 71.4% (5/7) — 2 remaining (hujson library limitations)

**Session progress: 77.3% → 97.8% (+20.5 percentage points, +67 files)**

## Plan to 100% (414/414)

### Phase 1: Parity Suite Fixes (3 files → 0 code changes to cfv)

**Task P1: Pass `--no-config` to prettier in parity suite**
- File: `~/.cfv-parity/run.mjs`
- Change: Add `config: false` to prettier format options (or CLI `--no-config`)
- Fixes: 2 argo-cd JSON files (.prettierrc tabWidth:4 no longer affects comparison)

**Task P2: Skip JSONC files that hujson can't parse**
- File: `~/.cfv-parity/run.mjs`
- Change: In JSONC loop, try hujson parse (or attempt cfv format). If cfv returns
  error, skip the file (same pattern as JSON.parse skip for invalid JSON).
- Fixes: 1 JSONC file with JSON5 unquoted keys

### Phase 2: Quick Fixes (2 files)

**Task F1: JSON number trailing zero normalization**
- Problem: hujson preserves literal `-9876.543210`. Prettier outputs `-9876.54321`.
- Spec: JSON numbers should be normalized (no trailing zeros after decimal point).
- Design: In `jsonfmt/json.go` after `FormatValue` returns, walk the output bytes
  and normalize number literals. Or: in hujson CST, find Literal nodes that are
  numbers with trailing zeros and trim them.
- Approach: Post-process the output bytes with a regex or scanner that finds
  decimal numbers with trailing zeros: `(\.\d*[1-9])0+([,\s\n\]\}])` → `$1$2`.
  Must NOT affect numbers inside string values.
- File: `pkg/formatter/jsonfmt/json.go`

**Task F2: JSONC single-quote to double-quote conversion**
- Problem: hujson preserves single-quoted string literals in output.
- Spec: JSONC uses double quotes (like JSON). Single quotes are technically not
  valid JSONC but hujson accepts them.
- Design: After `Format` returns, scan the output bytes for single-quoted strings
  and convert to double quotes. Same algorithm as YAML `convertFlowQuotes` —
  find quote boundaries, convert delimiters, handle escapes.
- File: `pkg/formatter/jsoncfmt/jsonc.go`

### Phase 3: TOML Edge Cases (1 file, 2 issues)

**Task T1: Blank lines between entries in sub-table groups**
- Problem: hugo.toml has blank lines between entries within `[[build.cachebusters]]`
  sections. Taplo removes them (no blank lines between entries in repeated
  array-table sections).
- Design: In `Print()` loop, when processing `GroupBlank` between entries, suppress
  blank lines when we're inside a sub-table group AND the previous table key equals
  the parent's key (repeated array table). Track `lastTableKey` — if blank is between
  two entries under the same array-table, suppress it.
- File: `pkg/formatter/tomlfmt/printer.go`

**Task T2: Isolated comment column preservation**
- Problem: hugo.toml `date = ['date']` has a comment at column 53 in source. Taplo
  preserves it. cfv normalizes to single space.
- Spec: Taplo preserves source comment column for isolated entries (not in a group
  of 2+) when the source column is wider than minimum.
- Design: In `findAlignedCommentColumns`, for runs of length 1, preserve the source
  column (from `inlineCommentColumn`) instead of returning 0.
- CAUTION: Earlier attempt at this caused regression (many files got source columns
  preserved that should be normalized). Need to understand EXACTLY when taplo
  preserves vs normalizes. Hypothesis: taplo preserves when column > minimum AND
  the column is "intentional" (not just random spacing from the original author).
  May need to investigate taplo source code.
- File: `pkg/formatter/tomlfmt/printer.go`

### Phase 4: YAML Major Features (2 files)

**Task Y1: Flow sequence line-width expansion**
- Problem: Flow sequences (`[...]`) exceeding 80 chars stay on one line.
- Spec: Prettier wraps them into multi-line flow (indented `[\n  elem,\n  ...\n]`).
- Design:
  1. In `printFormatted`, after all other passes, scan for TokFlow tokens that are
     flow SEQUENCES (start with `[`) and exceed `maxLineWidth`.
  2. Parse the flow sequence content into elements (split by comma at depth 0 —
     same algorithm as TOML's `splitByComma`).
  3. Replace the single-line TokFlow with multiple tokens: indent + `[\n` +
     per-element `indent+2 + elem + ",\n"` + indent + `]`.
  4. Alternatively: convert to block sequence (`- elem` style) which is what
     prettier actually does for YAML.
- Prettier's actual behavior: converts flow `[a, b, c]` to block:
  ```yaml
  - a
  - b
  - c
  ```
  This is a flow-to-block conversion, not multi-line flow.
- Complexity: HIGH. Requires understanding the context (is this a mapping value?
  what's the parent indent?). Must handle nested flows, quoted strings with commas,
  etc.
- File: `pkg/formatter/yamlfmt/printer.go`

**Task Y2: Multiline scalar prose wrapping**
- Problem: Long plain scalars stay on one line. Prettier wraps at 80 chars.
- Spec: Prettier's `proseWrap: "preserve"` (default for YAML) does NOT wrap.
  But `proseWrap: "always"` wraps at print width. Need to verify which mode
  prettier uses for YAML by default.
- Design: If prettier wraps by default, add a post-processing pass that finds
  TokValue tokens exceeding line width and inserts line breaks at word boundaries.
  The continuation lines get the same indent as the first line.
- Complexity: MEDIUM. Must preserve YAML scalar semantics (newlines in plain scalars
  become spaces when parsed). Must not break inside quoted strings.
- File: `pkg/formatter/yamlfmt/printer.go`

### Execution Order

1. P1, P2 (suite fixes — 0 risk, instant gains)
2. F1, F2 (quick cfv fixes — low risk, well-defined)
3. T1, T2 (TOML edge cases — medium risk, 1 file)
4. Y1 (YAML flow expansion — high complexity, most value)
5. Y2 (YAML prose wrap — medium complexity, verify prettier default first)

### Success Criteria

- Overall parity: 414/414 (100%) or 413/414 (99.8% — if Y2 turns out to be
  proseWrap:preserve which means prettier doesn't actually wrap)
- All 22 test packages pass
- golangci-lint: 0 issues
- Each task: plan → implement → test → deep code review

### Additional Fix: JSONC MaxLineWidth Default Resolution ✅

**Root cause:** JSONC `Format()` didn't resolve `MaxLineWidth: 0` to the default 80.
When called with zero (from `--no-config`), inline checks were disabled, expanding
everything unconditionally.

**Fix:** Added default resolution at top of `Format()`: `if opts.MaxLineWidth == 0 {
opts.MaxLineWidth = defaults.MaxLineWidth }`.

**Insight from prettier source:** Prettier's JSON uses the JS printer (estree) with
full group/break semantics, NOT the simple JSON-stringify printer. The JSON-stringify
printer always expands. This explains why package.json gets always-expanded formatting
while other JSON files get collapse/expand based on width.

### Additional Fixes This Session (after Task D)

**Task E: YAML Block Scalar Content Indent Normalization** ✅
- Normalize block scalar content to parentIndent + indentWidth
- Added trimBlockScalarTrailingBlanks for clip-chomped scalars
- Added collapseConsecutiveBlankLines (token-level) for inter-element blanks

**Task F: YAML Flow Bracket Spacing** ✅
- Added `[` and `]` handling to `addFlowMappingPadding`
- Removes space after `[`, removes space before `]`

**Task G: YAML Comment Spacing After Colon** ✅
- Tokenizer emits TokSpace for spaces between colon and comment
- Normalizer removes TokSpace when preceded by TokColon (avoids double-space)

**Task H: TOML Array-With-Comments Formatting** ✅
- Removed premature verbatim bailout for arrays containing comments
- Arrays now proceed to printArrayMultiline which handles comments correctly

### Remaining Issues (diminishing returns)

Each remaining failure is a distinct edge case:
- TOML: comment alignment (deferred), inline comment in line-width calc, blank lines
- YAML: AST depth miscalculation for nested sequences, multiline wrapping, doc markers
- JSON: .prettierrc mismatch (unfixable), number formatting
- JSONC: trailing commas, various

**Getting to 99% requires ~19 more fixes, each touching different subsystems. Many
require architectural changes (two-pass printing, comment-width in line calculations)
that are high-risk for diminishing returns.**

## Task: YAML Sequence Indent After Blank Lines (RCA + Fix)

### Root Cause Analysis

**Symptom:** After a sequence item with nested content followed by 2+ blank lines,
the NEXT sequence item gets double-indented (4 spaces instead of 2).

**Root cause chain:**

1. Tokenizer fix (Task C) emits zero-width `TokIndent("")` at every line start,
   including blank lines.

2. `reindentTokens` has two branches for indent tokens:
   - Structural (has ASTDepth ≥ 0): computes correct indent from AST metadata
   - Non-structural: applies `oldIndent + lastDelta` (shift by parent's change)

3. Blank-line indent tokens are non-structural (annotate skips them). They enter
   the `else` branch and get `lastDelta` applied. If the preceding structural line
   was deeply indented (e.g., `required: true` at indent 6, delta=+2 from original 4),
   blank-line indents become `0 + 2 = 2` (non-zero).

4. `collapseConsecutiveBlankLines` removes excess blank lines by suppressing
   TokNewline tokens when count > 2. But it only suppresses zero-width TokIndent
   tokens (`len(tok.Raw) == 0`). The now-non-zero blank-line indents (`"  "`) pass
   through.

5. After collapse removes the TokNewline (the actual blank line), the non-zero
   blank-line TokIndent remains adjacent to the NEXT line's TokIndent. Serialization
   concatenates them: `"  " + "  " = "    "` → 4 spaces.

### Design

**Fix location:** `reindentTokens` in `pkg/formatter/yamlfmt/printer.go`

**Change:** In the `else` branch (non-structural indent), check if the token is
followed by TokNewline (making it a blank-line indent). If so, keep it at 0
regardless of `lastDelta`. Blank lines should never have visible whitespace.

**Why this is correct:**
- Blank lines have no content — indenting them is meaningless
- `serializeWithStrip` already strips trailing whitespace from blank lines anyway
- Keeping them at 0 prevents the concatenation bug in `collapseConsecutiveBlankLines`
- The `lastDelta` value is preserved for the NEXT non-blank continuation line

**Why NOT fix in collapseConsecutiveBlankLines:**
- The collapse function shouldn't need to understand indent widths
- The root issue is that reindent produces nonsensical whitespace on blank lines
- Fixing at the source (reindent) prevents the problem class entirely

### Test Strategy

1. Minimal reproduction: `body:\n- type: one\n  validations:\n    required: true\n\n\n- type: two`
   → second item must get indent 2, not 4
2. Single blank line: still produces correct indent (no regression)
3. No blank line: still correct
4. Full parity files: 3 ISSUE_TEMPLATE files must match prettier
5. All existing unit tests pass (no regressions in other indent behavior)

**Symptom:** cfv uses single quotes in flow sequences (`['a', 'b']`), prettier
normalizes to double quotes (`["a", "b"]`).

**Root cause:** Flow collections (`[...]` and `{...}`) are tokenized as a single
opaque `TokFlow` token. `applyQuoteStyle` only processes `TokValue` tokens (block
scalars). It never sees the quoted scalars inside flow collections.

**Verified:**
- Block scalars DO get quote-converted: `- 'hello'` → `- "hello"` ✅
- Flow scalars do NOT: `['a', 'b']` stays as `['a', 'b']` ❌

**Design options:**

1. **Modify `applyQuoteStyle` to also process `TokFlow` tokens** — scan the raw bytes
   for quoted scalars, apply conversion in-place. This preserves the opaque tokenization
   while fixing the output.

2. **Break flow collections into fine-grained tokens** — fundamentally change the tokenizer
   to emit individual tokens for brackets, commas, and values inside flows. High risk,
   touches many consumers.

3. **Add a separate `convertFlowQuotes` pass** — specific function that regex-replaces
   quotes in TokFlow tokens. Simple but fragile (regex in YAML is dangerous).

**Chosen: Option 1** — modify `applyQuoteStyle` to handle `TokFlow` tokens. Scan raw
bytes for quoted scalars (respecting nesting depth so we don't convert inside nested
strings). For each single-quoted scalar found, apply the same conversion logic.

**Algorithm for flow token quote conversion:**
```
For each TokFlow token:
  Walk bytes, tracking:
    - Whether we're inside a single/double quoted string
    - Nesting depth ([ and { increase, ] and } decrease)
  When we find a single-quoted scalar at depth >= 1:
    - If QuoteStyle == QuoteDouble: replace ' delimiters with "
    - Handle '' escapes → convert to plain char (no escape needed in double quotes)
  When we find a double-quoted scalar at depth >= 1:
    - If QuoteStyle == QuoteSingle: replace " delimiters with '
    - Handle \" escapes → not needed in single quotes
```

**Edge cases to handle:**
- Quoted values containing the OTHER quote char: `'it"s here'` → need to handle
- Escaped quotes: `''` in single quotes (literal '), `\"` in double quotes
- Nested flow collections: `[['a'], 'b']` — only convert at any depth
- Values that NEED quotes (contain special chars): must stay quoted, just change style

**Simplification:** prettier only converts simple scalars that don't contain escape
sequences or the target quote character. We can apply the same restriction — only
convert quotes where `isSimpleQuoted` would pass AND the content doesn't contain the
target quote.

**Affected Files:**
- `pkg/formatter/yamlfmt/printer.go` — extend `applyQuoteStyle` or add companion function

**Test Strategy:**
1. `['a', 'b', 'c']` → `["a", "b", "c"]` (basic conversion)
2. `{"key": 'value'}` → `{"key": "value"}` (flow mapping)
3. `['it''s']` → stays `['it''s']` (contains escaped quote, can't convert)
4. `["has 'quotes'"]` → stays `["has 'quotes'"]` (content has single quotes)
5. `[['nested'], 'outer']` → `[["nested"], "outer"]` (nested works)
6. Existing tests still pass
7. Parity check on the 4 failing files

### Task E: YAML Comment Indentation (8 files)

**Symptom:** Comment indentation mismatches — cfv indents comments differently
from prettier in certain structures.

### Task F: TOML Comment Alignment (4 files) — DEFERRED

**Problem:** taplo generates column-aligned inline comments across consecutive entries.
cfv only preserves already-aligned columns from the source.

**Why deferred:** The architectural challenge is that column alignment requires knowing
the formatted output width of each entry's `key = value` portion BEFORE printing.
This requires either a two-pass approach or pre-computation of formatted lengths.
Both are significant architectural changes for 4 files of improvement.

**Impact:** 4 files (0.97% of total). Not worth the complexity/risk at this point.
Can revisit if we're within 4 files of 99% after other fixes.

### Task G: Update Parity Suite — Skip Invalid JSON (13 files)

**Symptom:** Files that fail `json.Valid()` (trailing commas, hex numbers, single
quotes, etc.) — prettier "fixes" them but cfv correctly rejects them. These should
be excluded from comparison.

### Task H: Final Parity Run

Run suite, verify >= 99%.

---

## Architecture Notes

### Prettier JSON Rules (verified empirically)

| Rule | Behavior |
|------|----------|
| Arrays collapse | ALWAYS if line ≤ 80 chars (source format irrelevant) |
| Objects preserve source | Expanded stays expanded; inline stays inline |
| Print width | indent + keyName + ": " + value ≤ 80 |
| Bracket spacing | `{ key: val }` with spaces; `{}` empty no spaces |
| Complexity heuristic | 2+ complex children → expand (NOT IMPLEMENTED) |

### Taplo TOML Rules (verified empirically)

| Rule | Behavior |
|------|----------|
| Arrays collapse | ALWAYS if line ≤ 80 (source format irrelevant) |
| Arrays expand | ALWAYS if line > 80 |
| Inline table arrays | Expand when total inline table line > 80 |
| Trailing comma | Only in expanded arrays; never in inline |
| Comment alignment | Preserves column-aligned runs of 2+ entries |

### YAML Formatter Architecture

```
tokenize → annotate (AST metadata) → reindentTokens → applyQuoteStyle → print
```

Key: `TokIndent` tokens at every line start enable `reindentTokens` to normalize
indentation based on AST depth.
