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

## Phase 1: JSONC Formatter (hujson) ✅

**Completed**: 2026-07-13 | **Commit**: dfb2dae
**Effort**: 1 session | **Risk**: Low

### What hujson gives us

- Full CST (`Value` with `BeforeExtra`/`AfterExtra` preserving comments and whitespace)
- `Parse()` for lossless parsing of JSON and JSONC
- `Pack()` for serialization from CST
- `IsStandard()` to detect whether input is pure JSON

### Design decision: custom CST walker, not hujson's Format()

hujson's `Format()` uses hardcoded tab indent with value alignment (padding
after colons to align values with the longest key). This alignment is
incompatible with configurable indent width — replacing tabs with spaces
produces non-idempotent output because the alignment math changes.

Solution: skip `Format()` entirely. Custom `formatState` walker sets
`BeforeExtra`/`AfterExtra` on each CST node for clean indentation. ~300
lines, deterministic, idempotent by construction.

### Delivered

- `pkg/formatter/jsoncfmt/jsonc.go` — formatter implementation
- `pkg/formatter/jsoncfmt/jsonc_test.go` — tests + fuzz
- 7 fixture pairs + 1 fuzz corpus entry
- Registered in `pkg/filetype/formatters.go`
- Fuzz: 10M+ executions, zero failures
- CLI functional test: tsconfig.jsonc, settings.jsonc, .eslintrc.json all format correctly
- Pipeline: 91.6% coverage, 0 lint issues

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

## Phase 4: TOML CST Tokenizer + Printer

**Effort**: 7-8 days
**Risk**: Medium — TOML grammar has genuine complexity in string boundaries
**Dependency**: pelletier/go-toml/v2 for validation only. No `unstable` package usage.
**Competition**: taplo (Rust). Must format every real-world TOML file correctly — no one should miss taplo because cfv can't handle their file.

### Architecture

```
Validation:  pelletier/go-toml/v2 Unmarshal (semantic correctness)
Formatting:  our tokenizer → token stream → printer (whitespace normalization)
```

The tokenizer is format-only. It classifies tokens and preserves boundaries. It does NOT:
- Decode escape sequences or values
- Resolve dotted keys into table hierarchy
- Validate semantics (pelletier does this)
- Build a document model

### Why custom tokenizer (not pelletier's unstable package)

- `unstable` package is explicitly labeled "does not meet backward compatibility guarantees"
- Depending on it for a core feature of a production tool is unacceptable architecture
- The `unstable` parser's `Raw` only covers 62% of source bytes (gaps in whitespace, table headers)
- A format-only tokenizer is fundamentally simpler than a full parser — no semantic model needed

### Why not WASM taplo

- +7-9MB binary size (50% increase) for one formatter
- Requires Rust toolchain in CI for .wasm compilation
- Introduces new dependency class (WASM runtime) used by nothing else
- Build complexity tax for contributors

### Token types

```
CommentToken       // # through end of line
WhitespaceToken    // spaces, tabs (not newlines)
NewlineToken       // \n or \r\n
TableHeaderToken   // [key] or [[key]] (including brackets)
KeyToken           // bare key, quoted key, or dotted key sequence
SeparatorToken     // = with optional surrounding whitespace
ValueToken         // string, number, bool, datetime, array, inline table
                   // (opaque — content preserved verbatim)
```

Values are treated as opaque tokens. The tokenizer tracks boundaries (opening/closing quotes, brackets, braces) but doesn't interpret content. This sidesteps most complexity.

### Hard parts (must get right)

1. **Multiline strings** (`"""..."""` and `'''...'''`) — closing sequence detection
2. **Inline tables** — nested brace counting `{a = {b = c}}`
3. **Multiline arrays** — bracket counting across lines with comments interspersed
4. **Comments after values** — `key = "value" # comment` boundary detection
5. **Dotted quoted keys** — `"a.b"."c.d" = value`

### Formatting operations on token stream

- **Normalize separator**: Ensure `key = value` spacing
- **Indent**: Apply configured indent to key-value pairs within tables
- **SortKeys**: Sort key-value token groups within table scope, preserve attached comments
- **Multiline values**: Preserved verbatim (content not modified)
- **FinalNewline/LineEnding**: Normalize newline tokens

### Tasks

- [ ] 4.1: Implement tokenizer in `pkg/formatter/tomlfmt/tokenizer.go`
  - Lex TOML source into token stream
  - Handle all string types (basic, literal, multiline basic, multiline literal)
  - Handle inline tables and multiline arrays (brace/bracket counting)
  - Every byte of input accounted for in exactly one token
  - Fuzz against pelletier: if pelletier accepts it, our tokenizer must not choke

- [ ] 4.2: Implement printer in `pkg/formatter/tomlfmt/printer.go`
  - Walk token stream, emit with normalized formatting
  - Normalize separator spacing
  - Apply indentation within table scopes
  - Implement SortKeys (sort key-value groups, preserve comment attachment)

- [ ] 4.3: Replace `toml.go` Format function
  - Keep pelletier for validation (`toml.Unmarshal`)
  - Replace line-oriented code with: validate → tokenize → format → print
  - Remove all verbatim-preservation heuristics
  - SortKeys actually works

- [ ] 4.4: Tests
  - All existing fixtures produce identical output
  - New fixtures: multiline strings, inline tables, dotted keys, SortKeys, arrays of tables
  - Fuzz: 45s minimum, zero failures
  - Stress test: format every .toml in the Cargo ecosystem (download top crates' Cargo.toml files)

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
Phase 1: JSONC (hujson)                          ✅ done (dfb2dae)
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

### 2026-07-13: pelletier/go-toml/v2 `unstable` rejected

- Package explicitly labeled "does not meet backward compatibility guarantees"
- Depending on it for a core production feature is unacceptable regardless of de-facto stability
- The `unstable` parser's `Raw` field only covers 62% of source bytes — gaps in whitespace, table headers
- Decision: use pelletier only for validation (stable `Unmarshal`), write own tokenizer for formatting

### 2026-07-13: WASM taplo rejected

- Would add +7-9MB to binary (50% increase) for a single formatter
- Requires Rust toolchain in CI for .wasm compilation
- Introduces new dependency class (WASM runtime via wazero) used nowhere else
- Build complexity tax disproportionate to benefit
- Only format where this pattern would apply (no other Rust-only formatters needed)

### 2026-07-13: Custom TOML tokenizer chosen

- Format-only tokenizer — classifies tokens, preserves boundaries, does not interpret values
- Fundamentally simpler than a full parser (no semantic model, no value decoding)
- pelletier handles validation, our tokenizer handles formatting
- Competition is taplo (Rust). Bar: no one reaches for taplo because cfv can't handle their file.
- Estimated 7-8 days including fuzz hardening for multiline string edge cases
- No Go library exists that provides format-preserving TOML (confirmed via exhaustive search)

### 2026-07-13: Contributing to pelletier deferred

- Comment preservation was deliberately removed in v2 (existed in v1)
- `KeepComments` flag on unstable parser shows awareness but not commitment
- Filing a feature request for something intentionally killed is tone-deaf
- If pelletier ships a stable comment-preserving API in the future, we can adopt it then
