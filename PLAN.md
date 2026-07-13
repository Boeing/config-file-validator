# PLAN.md — Formatter Architecture: CST-Based Rewrite

**Date**: 2026-07-13
**Context**: Investigation revealed that silent bail-outs (`return src, nil`) in Properties, INI, and TOML formatters are an architectural smell. The root cause is a validate-then-transform pattern where the formatter operates on raw text disconnected from semantic understanding. Every major formatter (gofmt, prettier, rustfmt, black) uses parse→model→print, which is correct by construction. This plan migrates all formatters to CST-based parsing with zero bail-outs.

**Supersedes**: Round 2 tasks (Fix 6, Fix 7) in previous plan. Those bugs are symptoms of the architecture we're replacing.

---

## Current State

| Format | Architecture | Bail-outs | CST Library Available |
|--------|-------------|-----------|----------------------|
| HCL | ✅ CST (hclwrite token stream) | 0 | hashicorp/hcl/v2 |
| JSON | ✅ Semantic (tidwall/pretty) | 0 | encoding/json + tidwall/pretty |
| YAML | ✅ CST (goccy/go-yaml AST) | 1 (empty file) | goccy/go-yaml |
| JSONC | ❌ No formatter exists | N/A | tailscale/hujson (**already in go.mod**) |
| XML | ⚠️ DOM (helium) | 2 (ErrSkipped) | lestrrat-go/helium (has bugs, issues filed) |
| Properties | ❌ Line-oriented | 4 | None exists — must write CST |
| INI | ❌ Line-oriented | 3 | None exists — must write CST |
| TOML | ❌ Line-oriented | 3 | pelletier/go-toml/v2 `unstable` (read-only AST) |
| ENV | ✅ Line-oriented (format is trivial) | 0 | N/A (format too simple to need CST) |

---

## Design Principles

1. **No silent bail-outs.** Either format the file correctly or return an error/ErrSkipped with a reason. Never return input unchanged pretending it worked.
2. **Parse once, understand fully, print deterministically.** The formatter must understand the file's structure (including escapes, continuations, multiline values, comments) at the token level.
3. **Idempotent by construction.** Same CST → same output. No runtime idempotency checks.
4. **Comment preservation is mandatory.** Comments are first-class tokens in the CST, not afterthoughts.
5. **Use existing libraries where they provide CST.** Only write custom parsers where no library exists.
6. **Validate with the validation library. Format with the CST.** These may be different libraries.

---

## Phase 1: JSONC Formatter (hujson)

**Effort**: 1 day
**Risk**: Low — library does the heavy lifting
**Dependency**: tailscale/hujson (already in go.mod)

### What hujson gives us

- Full CST (`Value` with `BeforeExtra`/`AfterExtra` preserving comments and whitespace)
- `Format()` method — idempotent, comment-preserving
- `Object.Members` is a public slice — trivially sortable for SortKeys
- Handles JSON AND JSONC (comments + trailing commas) with one parser
- If input is standard JSON, output remains standard JSON

### Limitations to address

- `Format()` uses hardcoded 1-tab indent — no configurable indent width
- SortKeys needs custom implementation (sort `Object.Members` recursively)

### Tasks

- [ ] 1.1: Create `pkg/formatter/jsoncfmt/` package
  - Parse with `hujson.Parse()`
  - If `opts.SortKeys`: recursively sort `Object.Members` by key
  - Call `value.Format()` for standard formatting
  - Post-process indent: replace leading tabs with configured indent string
  - Apply `FinalNewline` and `LineEnding` normalization
  - No bail-outs, no re-validation
  - **Files**: `pkg/formatter/jsoncfmt/jsonc.go`

- [ ] 1.2: Register JSONC formatter in `pkg/filetype/formatters.go`
  - Map `"jsonc"` → `jsoncfmt.Formatter{}`

- [ ] 1.3: Tests
  - Fixture-based tests (basic, comments, trailing commas, nested, sorted)
  - Idempotency test on all fixtures
  - Fuzz test (same contract: no panics, if succeeds then idempotent)
  - Verify standard JSON input produces standard JSON output
  - **Files**: `pkg/formatter/jsoncfmt/jsonc_test.go`, `testdata/`

- [ ] 1.4: Pipeline verification
  - `go test ./...`, `golangci-lint run`, coverage ≥ 90%

---

## Phase 2: Properties CST Parser

**Effort**: 2-3 days
**Risk**: Low — grammar is trivial, we're already 80% there
**Dependency**: None new. Keep magiconair/properties for validation only.

### Why custom CST

No Go library provides format-preserving Properties parsing. Our current `propfmt` already has `findSeparator` that walks characters and handles escapes — it's 80% of a lexer. The gap: it works at line granularity instead of token granularity, which causes it to miss continuation semantics and escape interactions.

### Token types

```
CommentToken     // # or ! prefix through end of line
BlankToken       // empty line
KeyToken         // key characters (with escape sequences preserved)
SeparatorToken   // =, :, or first whitespace between key and value
ValueToken       // value characters (with escape sequences preserved)
ContinuationToken // trailing \ + newline + leading whitespace on next line
NewlineToken     // \n or \r\n
```

### Design

```go
type Token struct {
    Kind    TokenKind
    Raw     string   // original bytes exactly as they appeared
    Value   string   // decoded value (for Key/Value tokens: unescaped)
}

type Entry struct {
    LeadingComments []Token  // comment lines preceding this entry
    Key             Token
    Separator       Token
    Value           []Token  // value + continuation tokens
    InlineComment   *Token   // rarely used in properties but possible
    Newline         Token
}

type File struct {
    Entries  []Entry
    Trailing []Token  // trailing comments/blanks after last entry
}
```

### Formatting operations on the CST

- **Normalize separator**: Replace `SeparatorToken.Raw` with ` = ` (or configured separator)
- **SortKeys**: Sort `File.Entries` by `Entry.Key.Value` (decoded key), preserving attached `LeadingComments`
- **Indent**: Not applicable for properties
- **FinalNewline/LineEnding**: Normalize `NewlineToken`s

The printer walks `File` and emits `Token.Raw` for every token, except for the Separator which gets normalized. This is correct by construction — we only modify what we intend to, everything else is verbatim.

### Tasks

- [ ] 2.1: Implement lexer in `pkg/formatter/propfmt/lexer.go`
  - Tokenize properties file into stream of tokens
  - Handle: escape sequences (`\n`, `\t`, `\uXXXX`, `\\`), continuation (`\` + newline), comment lines (`#`, `!`), separator detection (first unescaped `=`, `:`, or whitespace)
  - Every byte of input is accounted for in exactly one token
  - **Files**: `pkg/formatter/propfmt/lexer.go`

- [ ] 2.2: Implement parser in `pkg/formatter/propfmt/parser.go`
  - Build `File` structure from token stream
  - Associate comments with following entries
  - Track continuation tokens as part of value
  - **Files**: `pkg/formatter/propfmt/parser.go`

- [ ] 2.3: Implement printer in `pkg/formatter/propfmt/printer.go`
  - Walk `File`, emit tokens
  - Normalize separator spacing
  - Implement SortKeys (sort entries, preserve comment attachment)
  - Apply FinalNewline and LineEnding
  - **Files**: `pkg/formatter/propfmt/printer.go`

- [ ] 2.4: Replace `properties.go` Format function
  - Keep `magiconair/properties` for validation (catch invalid escape sequences the lexer might accept)
  - Replace line-oriented code with: validate → lex → parse → transform → print
  - Remove all `return src, nil` bail-outs
  - Remove idempotency check (correctness is structural)
  - **Files**: `pkg/formatter/propfmt/properties.go`

- [ ] 2.5: Tests
  - All existing fixtures must produce identical output
  - New fixtures: continuation lines, escaped keys, SortKeys with comments
  - Fuzz: 45s minimum, zero failures
  - **Files**: `pkg/formatter/propfmt/properties_test.go`, `testdata/`

- [ ] 2.6: Pipeline verification

---

## Phase 3: INI CST Parser

**Effort**: 2-3 days
**Risk**: Low — similar complexity to Properties
**Dependency**: None new. Keep gopkg.in/ini.v1 for validation only.

### Token types

```
CommentToken      // # or ; prefix through end of line
BlankToken        // empty line
SectionToken      // [section-name]
KeyToken          // key characters
SeparatorToken    // = or :
ValueToken        // value characters (may include quotes that the parser preserves verbatim)
NewlineToken      // \n or \r\n
```

### Design

```go
type Section struct {
    LeadingComments []Token
    Header          Token     // [section-name] — nil for default section
    Entries         []Entry
}

type Entry struct {
    LeadingComments []Token
    Key             Token
    Separator       Token
    Value           Token
    InlineComment   *Token
    Newline         Token
}

type File struct {
    Sections []Section
    Trailing []Token
}
```

### Formatting operations

- **Normalize separator**: Replace `SeparatorToken.Raw` with ` = ` (or configured)
- **Indent**: Prepend configured indent to Key/Comment tokens within sections
- **SortKeys**: Sort `Section.Entries` by key within each section
- **Quoted values**: `Value.Raw` is preserved verbatim — we never interpret quotes, we just carry them through

### Tasks

- [ ] 3.1: Implement lexer in `pkg/formatter/inifmt/lexer.go`
  - Tokenize INI file
  - Handle: section headers, comments (# and ;), key-value pairs, escaped characters
  - No interpretation of quoted values — they're opaque value tokens
  - **Files**: `pkg/formatter/inifmt/lexer.go`

- [ ] 3.2: Implement parser in `pkg/formatter/inifmt/parser.go`
  - Build `File` → `Section` → `Entry` structure
  - Associate comments with following entries/sections
  - **Files**: `pkg/formatter/inifmt/parser.go`

- [ ] 3.3: Implement printer in `pkg/formatter/inifmt/printer.go`
  - Walk `File`, emit tokens
  - Normalize separator, apply indent
  - Implement SortKeys (within sections)
  - Apply FinalNewline and LineEnding
  - **Files**: `pkg/formatter/inifmt/printer.go`

- [ ] 3.4: Replace `ini.go` Format function
  - Keep `gopkg.in/ini.v1` for validation
  - Remove all `return src, nil` bail-outs
  - **Files**: `pkg/formatter/inifmt/ini.go`

- [ ] 3.5: Tests
  - All existing fixtures must produce identical output
  - New fixtures: quoted values, SortKeys, keys with special characters
  - Fuzz: 45s minimum, zero failures
  - **Files**: `pkg/formatter/inifmt/ini_test.go`, `testdata/`

- [ ] 3.6: Pipeline verification

---

## Phase 4: TOML CST Parser

**Effort**: 5-7 days
**Risk**: Medium — TOML grammar is complex (nesting, multiline strings, inline tables, dotted keys)
**Dependency**: pelletier/go-toml/v2 `unstable` package (read-only AST with comments)

### Approach

pelletier/go-toml/v2's `unstable.Parser` provides a read-only AST that preserves comments and positions. It has node types for `KeyValue`, `Table`, `ArrayTable`, `Array`, `InlineTable`, and `Comment`. What it lacks is a serializer.

We write a custom printer that walks the `unstable` AST and emits formatted output. The AST gives us structural understanding (scope, nesting, multiline boundaries). The printer handles indentation and separator normalization.

**Trade-off**: The `unstable` package is explicitly marked as not having backward-compatibility guarantees. However:
- It's been stable in practice for 2+ years
- We pin exact versions in go.mod
- The alternative (writing our own TOML lexer/parser) is far more effort and maintenance burden
- If the API breaks in a future version, the fix is adapting our printer to the new API, not rewriting the parser

### Token-level approach via unstable AST

```go
// The unstable parser gives us:
type Node struct {
    Kind    Kind    // KeyValue, Table, ArrayTable, Comment, etc.
    Raw     Range   // byte offsets into source
    Data    []byte  // raw bytes for this node
    Children []Node
}
```

We walk nodes, emit their raw bytes with formatting adjustments:
- `KeyValue` nodes: normalize spacing around `=`
- `Table`/`ArrayTable` nodes: emit header, indent children
- `Comment` nodes: preserve verbatim with indent
- Multiline values: detected by node boundaries, preserved verbatim
- SortKeys: sort `KeyValue` children within each `Table`, preserving attached comments

### Tasks

- [ ] 4.1: Implement AST walker and printer in `pkg/formatter/tomlfmt/printer.go`
  - Walk `unstable.Parser` output
  - Classify nodes, emit with formatting
  - Handle all TOML constructs: basic/literal strings, multiline strings, arrays, inline tables, dotted keys, datetime
  - **Files**: `pkg/formatter/tomlfmt/printer.go`

- [ ] 4.2: Implement SortKeys
  - Sort KeyValue nodes within table scope
  - Preserve comment attachment (comments before a key travel with it)
  - Handle dotted keys correctly (sort by first segment, preserve sub-key order)
  - **Files**: `pkg/formatter/tomlfmt/printer.go`

- [ ] 4.3: Replace `toml.go` Format function
  - Keep `pelletier/go-toml/v2` stable API for validation (`Unmarshal`)
  - Use `unstable.Parser` for CST access
  - Remove line-oriented code
  - Remove multiline preservation hacks
  - **Files**: `pkg/formatter/tomlfmt/toml.go`

- [ ] 4.4: Tests
  - All existing fixtures must produce identical output
  - New fixtures: multiline strings (basic + literal), inline tables, dotted keys, SortKeys, arrays of tables
  - Fuzz: 45s minimum, zero failures
  - **Files**: `pkg/formatter/tomlfmt/toml_test.go`, `testdata/`

- [ ] 4.5: Pipeline verification

---

## Phase 5: XML (Pending helium upstream fixes)

**Effort**: 1-2 days after helium fixes land
**Risk**: Low — helium already provides DOM, we just need bugs fixed
**Dependency**: lestrrat-go/helium (issues filed, awaiting response)

### Issues filed

1. **StripBlanks(true) treats entity-encoded and literal characters differently** — bug, causes idempotency failure
2. **Writer.Format(true) inserts indentation inside mixed-content elements** — bug, corrupts text content

### Plan

- **If helium fixes both**: Upgrade, remove `ErrSkipped` for mixed content (helium handles it correctly), delete fuzz corpus entry. Done.
- **If helium fixes StripBlanks only**: Upgrade. Keep `ErrSkipped` for mixed content as a documented scope limitation until Writer fix lands.
- **If helium is unresponsive (30+ days)**: Evaluate switching to `beevik/etree` for formatting with custom mixed-content-aware serializer (~80 lines). Keep helium for XSD validation only.
- **If we contribute fixes ourselves**: Fork, fix, PR upstream, use fork via `replace` directive until merged.

### Tasks (deferred until upstream response)

- [ ] 5.1: Upgrade helium when fixes land
- [ ] 5.2: Remove ErrSkipped for mixed content (if Writer fix lands)
- [ ] 5.3: Fuzz: 45s, zero failures
- [ ] 5.4: Pipeline verification

---

## Phase 6: YAML and ENV cleanup

**Effort**: < 1 day
**Risk**: Negligible

### YAML

The YAML formatter is already CST-based (goccy/go-yaml AST). Two issues:
- Empty/whitespace input returns `src, nil` → change to `ErrSkipped` or format to empty with final newline
- `IndentTabs` silently falls back to spaces → return error "YAML does not support tab indentation per spec"

### ENV

Already has zero bail-outs. No changes needed.

### Tasks

- [ ] 6.1: YAML empty input: return `ErrSkipped{Reason: "empty document"}` instead of silent return
- [ ] 6.2: YAML IndentTabs: return error instead of silent fallback
- [ ] 6.3: Tests and pipeline verification

---

## Execution Order

```
Phase 1: JSONC (hujson)                          — 1 day,  low risk, immediate value
Phase 2: Properties CST                          — 2-3 days, low risk
Phase 3: INI CST                                 — 2-3 days, low risk
Phase 6: YAML/ENV cleanup                        — <1 day, negligible risk
Phase 4: TOML CST                                — 5-7 days, medium risk
Phase 5: XML (blocked on helium upstream)        — 1-2 days when unblocked
```

Phases 1-3 can be done sequentially. Phase 6 can slot in anywhere. Phase 4 is the largest effort and benefits from patterns established in Phase 2-3. Phase 5 is externally blocked.

---

## Acceptance Criteria (overall)

- Zero `return src, nil` patterns across all formatters
- Zero silent bail-outs — every formatter either formats or returns an explicit error
- Fuzz 45s per formatter, zero failures, all 8+ formatters
- Full pipeline green, coverage ≥ 90%
- SortKeys works for: JSON ✅, JSONC (new), YAML ✅, Properties (new), INI (new), TOML (new)
- All existing fixtures produce identical output (no behavior regression)

---

## Decision Log

### 2026-07-13: CST-based rewrite over incremental fixes

- Silent bail-outs are architectural smell from validate-then-transform pattern
- Every major formatter uses parse→model→print; we should too
- Line-oriented approach hits ceiling at SortKeys, escape handling, continuation
- "One-stop shop" positioning requires first-class formatting, not secondary
- Go ecosystem has no CST libraries for Properties/INI — we must write our own
- TOML has pelletier's `unstable` AST (read-only, no serializer) — we write the printer
- JSONC is free via hujson (already in go.mod)
- XML is blocked on helium upstream bug fixes

### 2026-07-13: etree evaluated but not adopted (for now)

- beevik/etree handles entity/whitespace correctly where helium doesn't
- Same mixed-content limitation as helium (fundamental XML property)
- If helium is unresponsive on bug fixes, etree becomes the formatting backend
- Would add one dependency but eliminate the StripBlanks bug

### 2026-07-13: pelletier/go-toml/v2 `unstable` accepted despite API stability warning

- Has been stable in practice for 2+ years
- We pin versions — breaking changes are a version bump, not a surprise
- Alternative (write own TOML parser) is effort 7-8 and permanent maintenance
- Risk is manageable: if API breaks, we adapt the printer, not the parser
