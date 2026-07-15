# PLAN.md — Formatter Architecture: CST-Based Rewrite

**Date**: 2026-07-13
**Context**: Investigation revealed that silent bail-outs (`return src, nil`) in Properties, INI, and TOML formatters are an architectural smell. The root cause is a validate-then-transform pattern where the formatter operates on raw text disconnected from semantic understanding. Every major formatter (gofmt, prettier, rustfmt, black) uses parse→model→print, which is correct by construction. This plan migrates all formatters to CST-based parsing with zero bail-outs.

**Supersedes**: Round 2 tasks (Fix 6, Fix 7) in previous plan. Those bugs are symptoms of the architecture we're replacing.

---

## Current State

| Format | Architecture | Bail-outs | Status |
|--------|-------------|-----------|--------|
| HCL | ✅ CST (hclwrite token stream) | 0 | Done (pre-existing) |
| JSON | ✅ Semantic (tidwall/pretty) | 0 | Done (pre-existing) |
| YAML | ✅ CST (custom tokenizer) + AST (yaml.v3 Node) | 0 | **Done** |
| JSONC | ✅ CST (hujson + custom walker) | 0 | **Done (dfb2dae)** |
| TOML | ✅ CST (custom tokenizer/grouper/printer) | 0 | **Done (6166dcc)** — identical to taplo |
| XML | ✅ CST (custom tokenizer, tag-counting depth) | 0 | **Done** |
| Properties | ✅ CST (custom tokenizer/printer) | 0 | **Done** |
| INI | ✅ CST (custom tokenizer/parser/printer) | 0 | **Done** |
| ENV | ✅ Line-oriented (format is trivial) | 0 | Done (pre-existing) |

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

- [x] 2.1: Implement lexer in `pkg/formatter/propfmt/lexer.go`
  - Tokenize properties file into stream of tokens
  - Handle: escape sequences (`\n`, `\t`, `\uXXXX`, `\\`), continuation (`\` + newline), comment lines (`#`, `!`), separator detection (first unescaped `=`, `:`, or whitespace)
  - Every byte of input is accounted for in exactly one token
  - **Files**: `pkg/formatter/propfmt/lexer.go`

- [x] 2.2: Implement parser in `pkg/formatter/propfmt/parser.go`
  - Build `File` structure from token stream
  - Associate comments with following entries
  - Track continuation tokens as part of value
  - **Files**: `pkg/formatter/propfmt/parser.go`

- [x] 2.3: Implement printer in `pkg/formatter/propfmt/printer.go`
  - Walk `File`, emit tokens
  - Normalize separator spacing
  - Implement SortKeys (sort entries, preserve comment attachment)
  - Apply FinalNewline and LineEnding
  - **Files**: `pkg/formatter/propfmt/printer.go`

- [x] 2.4: Replace `properties.go` Format function
  - Keep `magiconair/properties` for validation (catch invalid escape sequences the lexer might accept)
  - Replace line-oriented code with: validate → lex → parse → transform → print
  - Remove all `return src, nil` bail-outs
  - Remove idempotency check (correctness is structural)
  - **Files**: `pkg/formatter/propfmt/properties.go`

- [x] 2.5: Tests
  - All existing fixtures must produce identical output
  - New fixtures: continuation lines, escaped keys, SortKeys with comments
  - Fuzz: 45s minimum (8.7M executions), zero failures
  - **Files**: `pkg/formatter/propfmt/properties_test.go`, `testdata/`

- [x] 2.6: Pipeline verification

### Delivered

- `pkg/formatter/propfmt/tokenizer.go` — lossless tokenizer (handles escapes, continuations, comments)
- `pkg/formatter/propfmt/printer.go` — CST grouper + printer (separator normalization, SortKeys with comment attachment)
- `pkg/formatter/propfmt/properties.go` — Format function: validate → tokenize → group → print (zero bail-outs)
- `pkg/formatter/propfmt/properties_test.go` — fixtures, idempotency, fuzz
- 7 fixture pairs: basic, comments, sorted, already_formatted, continuation, escaped_keys, sorted_comments
- Fuzz: 8.7M+ executions in 45s, zero failures
- Registered in `pkg/filetype/formatters.go`
- Pipeline: 91.9% coverage, 0 lint issues

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

- [x] 3.1: Implement lexer in `pkg/formatter/inifmt/lexer.go`
  - Tokenize INI file
  - Handle: section headers, comments (# and ;), key-value pairs, escaped characters
  - No interpretation of quoted values — they're opaque value tokens
  - **Files**: `pkg/formatter/inifmt/lexer.go`

- [x] 3.2: Implement parser in `pkg/formatter/inifmt/parser.go`
  - Build `File` → `Section` → `Entry` structure
  - Associate comments with following entries/sections
  - **Files**: `pkg/formatter/inifmt/parser.go`

- [x] 3.3: Implement printer in `pkg/formatter/inifmt/printer.go`
  - Walk `File`, emit tokens
  - Normalize separator, apply indent
  - Implement SortKeys (within sections)
  - Apply FinalNewline and LineEnding
  - **Files**: `pkg/formatter/inifmt/printer.go`

- [x] 3.4: Replace `ini.go` Format function
  - Keep `gopkg.in/ini.v1` for validation
  - Remove all `return src, nil` bail-outs
  - **Files**: `pkg/formatter/inifmt/ini.go`

- [x] 3.5: Tests
  - All existing fixtures must produce identical output
  - New fixtures: quoted values, SortKeys, keys with special characters, escaped keys, sort with escapes
  - Fuzz: 45s minimum (3.1M executions), zero failures
  - **Files**: `pkg/formatter/inifmt/ini_test.go`, `testdata/`

- [x] 3.6: Pipeline verification (96.6% coverage, 0 lint issues)

### Delivered

- `pkg/formatter/inifmt/lexer.go` — lossless tokenizer (sections, comments, key-value, CR/CRLF/LF)
- `pkg/formatter/inifmt/parser.go` — Section→Entry tree builder with comment association
- `pkg/formatter/inifmt/printer.go` — formatter (separator normalization, indent, SortKeys within sections)
- `pkg/formatter/inifmt/ini.go` — Format function: validate → tokenize → parse → print (zero bail-outs)
- `pkg/formatter/inifmt/ini_test.go` — fixtures, idempotency, fuzz, edge cases (CR, CRLF, whitespace-only)
- 8 fixture pairs: basic, comments, already_formatted, quoted_values, sort_keys, colon_separator, escaped_keys, sort_escaped
- Fuzz: 3.1M+ executions in 45s, zero failures
- Pipeline: 96.6% coverage, 0 lint issues

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

## Phase 5: XML CST Formatter ✅

**Status**: Done (a03e241 + Phase 9 fixes)
**Approach taken**: Custom tokenizer + tag-counting depth. helium used for validation only (syntax + XSD). The helium Writer bugs (StripBlanks, Format) were bypassed entirely — our CST printer handles formatting independently.

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

## Phase 6A: YAML CST Formatter (custom tokenizer)

**Effort**: 1-2 sessions
**Risk**: Medium — YAML grammar is complex but we only need token boundaries, not semantics
**Dependency**: goccy/go-yaml for validation only. Custom tokenizer for formatting.
**Reference**: prettier (via yaml-unist-parser) for expected output behavior.

### Why custom tokenizer

No Go library provides a lossless YAML token stream suitable for formatting:
- goccy/go-yaml: `Origin` field exists but whitespace boundaries are inconsistent (indent may be on preceding token's tail OR following token's head). `file.String()` is a reconstructive serializer, not a CST printer.
- go-yaml/yaml (v3): No CST at all. Decode/encode only.
- google/yamlfmt: Same decode/encode pattern with placeholder-comment hacks for blank-line preservation.

prettier's YAML plugin uses `yaml-unist-parser` (unist AST with source positions) and slices `originalText` for verbatim content. We take this further: a pure tokenizer where every byte is one token, and formatting is token manipulation.

### Architecture

```
Validation:  goccy/go-yaml Unmarshal (semantic correctness, duplicate keys, anchors)
Formatting:  custom tokenizer → token stream → printer (indent normalization)
```

The tokenizer classifies bytes into tokens. It does NOT:
- Decode escape sequences or values
- Resolve anchors/aliases
- Build a document model
- Validate semantics

### Token types

```
IndentToken        // leading spaces at start of line (the thing we modify)
NewlineToken       // \n or \r\n
CommentToken       // # through end of line (not including newline)
KeyToken           // bare key, quoted key (content opaque)
ColonToken         // : (with optional trailing space)
ValueToken         // scalar value content on same line as colon
DashToken          // - (sequence entry indicator)
DocStartToken      // ---
DocEndToken        // ...
TagToken           // !tag, !!type
AnchorToken        // &name
AliasToken         // *name
BlockScalarToken   // | or > plus header plus all content lines (opaque block)
FlowToken          // { ... } or [ ... ] (opaque — brace/bracket counted)
DirectiveToken     // %YAML, %TAG
SpaceToken         // horizontal whitespace not at line start (between tokens)
```

Key design decisions:
- **IndentToken is separate from SpaceToken** — indent is leading whitespace on a line, space is whitespace between tokens on the same line. Only IndentToken gets modified during formatting.
- **BlockScalarToken is opaque** — includes the header (|+, >-) and ALL content lines. Content indentation is relative to the scalar's indent and MUST NOT be modified independently.
- **FlowToken is opaque** — `{key: value, other: [1,2,3]}` is a single token. We don't reformat flow internals (same as prettier).
- **Values are opaque** — quoted strings, multiline plain scalars — content preserved verbatim.

### What the tokenizer needs to handle

1. **Line-start indent detection** — after every newline, consume spaces → IndentToken
2. **Comments** — `#` through EOL (not including the newline)
3. **Block scalar boundaries** — `|` or `>` followed by optional indicators, then content lines determined by indent level. First non-empty content line sets the content indent; everything at or above that indent (until indent drops) is part of the block.
4. **Flow collection boundaries** — `{`/`}` and `[`/`]` nesting count. Everything inside is one FlowToken.
5. **Quoted strings** — `'...'` and `"..."` with escape handling for double-quote. These are part of ValueToken or KeyToken.
6. **Plain scalars** — unquoted text that may span multiple lines (continuation lines more indented than key). Boundary detection: next line at same or lower indent starts a new entry.
7. **Sequence entry** — `- ` at appropriate indent
8. **Mapping key/value** — key followed by `:` followed by value
9. **Document markers** — `---` and `...` at column 0
10. **Directives** — `%` at column 0

### Formatting operations on token stream

- **Reindent**: Replace IndentToken.Raw with new indent (depth × targetWidth). This requires knowing the structural depth — derived from indent levels in the original token stream.
- **SortKeys**: Group tokens into "entries" (key + colon + value + nested content until indent drops), sort entries within a mapping scope.
- **FinalNewline/LineEnding**: Normalize NewlineTokens.
- **Block scalars**: When reindenting, block scalar content indent shifts by the same delta as the scalar's own indent. This maintains the relative indentation within the block.

### Grouping for SortKeys

To sort keys, we need to identify "entries" — a key-value pair and all its nested content:

```
Entry = IndentToken + KeyToken + ColonToken + [ValueToken | nested content until indent ≤ entry indent]
```

The grouper walks the token stream and builds:
```go
type Entry struct {
    LeadingComments []Token   // comments at this indent preceding the key
    Tokens         []Token   // all tokens from key through end of nested content
    Key            string    // decoded key for sort comparison
    Indent         int       // the indent level of this entry's key
}
```

Sorting reorders entries within a mapping, preserving comment attachment.

### Tasks

- [x] 6A.1: Implement tokenizer in `pkg/formatter/yamlfmt/tokenizer.go`
  - Lossless tokenization — 9.3M fuzz executions, zero failures
  - Handles: indent, comments, block scalars (opaque), flow collections (opaque), quoted strings, plain scalars, sequence entries, mapping pairs, document markers, directives, anchors, aliases, tags

- [x] 6A.2: Implement grouper in `pkg/formatter/yamlfmt/grouper.go`
  - Depth computation via stack-based indent tracking
  - Entry grouping for SortKeys with comment attachment

- [x] 6A.3: Implement printer in `pkg/formatter/yamlfmt/printer.go`
  - Indent normalization (depth × targetWidth)
  - Block scalar content indent shifting (delta-based)
  - SortKeys (recursive, with newline separation)
  - QuoteStyle (prettier's conservative approach — bail on escapes)
  - Inline comment space normalization

- [x] 6A.4: Replace yaml.go Format function
  - CST pipeline: validate → tokenize → format → print
  - Removed stability guard, removed AST manipulation code
  - All 14 existing fixtures pass

- [x] 6A.4.1: Switch validation from goccy/go-yaml to gopkg.in/yaml.v3
  - Completed in commits b4ab550 + 55ac455
  - goccy removed from go.mod, yaml.v3 used for validation + structural line map
  - Error messages: regex parse yaml.v3's `yaml: line N: msg` format
  - Position map: yaml.Node tree for schema validation
  - Multi-doc: yaml.NewDecoder loop with io.EOF check
  - All dead goccy imports and AST code removed

- [ ] 6A.5: Tests
  - All existing fixtures must produce identical output
  - Fuzz: 45s minimum, zero failures (with yaml.v3 rejecting pathological inputs)
  - Test against prettier output on real-world files

- [ ] 6A.6: Pipeline verification

### Competition

prettier. Our formatter must produce output that a prettier user would find acceptable. Not necessarily byte-for-byte identical (prettier has its own opinions on quoting, line-wrapping prose), but structurally equivalent: same indent, same key order when sorted, same comment placement, block scalars verbatim.

### Hard parts (must get right)

1. **Block scalar boundary detection** — the indent of the first content line sets the boundary. Must handle empty lines within block scalars (they don't terminate the scalar).
2. **Plain scalar continuation** — a plain value continues on the next line if that line is more indented than the mapping key. Must not split a multi-line plain scalar.
3. **Entry boundary for SortKeys** — "where does this entry end?" requires tracking indent. An entry ends when the next non-comment, non-blank token is at the same or lower indent.
4. **Block scalar reindent** — when the parent changes indent, block scalar content shifts by the same delta. Must not corrupt content.

---

## Phase 6B: YAML and ENV cleanup (post-CST)

**Effort**: < 1 day
**Risk**: Negligible
**Status**: ✅ Done (completed during Phase 6A prep)

**Effort**: < 1 day
**Risk**: Negligible

### YAML

The YAML formatter is already CST-based (goccy/go-yaml AST). Two issues:
- Empty/whitespace input returns `src, nil` → change to `ErrSkipped` or format to empty with final newline
- `IndentTabs` silently falls back to spaces → return error "YAML does not support tab indentation per spec"

### ENV

Already has zero bail-outs. No changes needed.

### Tasks

- [x] 6.1: YAML empty input: return `ErrSkipped{Reason: "empty document"}` instead of silent return
- [x] 6.2: YAML IndentTabs: return error instead of silent fallback
- [x] 6.3: Tests and pipeline verification

### Delivered

- Empty/whitespace input now returns `ErrSkipped` (no more silent `return src, nil`)
- `IndentTabs` returns explicit error: "tab indentation is not supported (YAML spec requires spaces)"
- Fixed `hasFormattableRoot` to reject documents with all-nil bodies (prevents formatter producing whitespace-only output that can't be re-formatted)
- Fuzz: 1.8M executions, zero failures
- Pipeline: 92.1% coverage, 0 lint issues

---

## Execution Order

```
Phase 1: JSONC (hujson)                          ✅ done (dfb2dae)
Phase 4: TOML CST                                ✅ done (6166dcc + 2c76b0a)
Phase 2: Properties CST                          ✅ done
Phase 3: INI CST                                 ✅ done
Phase 6: YAML/ENV cleanup                        ✅ done
Phase 6A: YAML CST formatter                     ✅ done (f4a18de + b4ab550 + 55ac455 + 6a5ba6f + Phase 9 fixes)
                                                    Custom tokenizer, yaml.v3 validation, anchor-safe sort,
                                                    recomputeDepths after sort, QuoteStyle idempotent.
Phase 5: XML CST formatter                       ✅ done (a03e241 + Phase 9 fixes)
                                                    Mixed content handled correctly, PI classification fixed.
Phase 9: Code review fixes                       ✅ done (9.1-9.11 all complete)
                                                    All 35 review findings addressed.
                                                    Pipeline: 90.3% coverage, vet/fmt/build clean.
Phase 7: Ephemeral CLI stress test (ALL formats) ✅ done
                                                    102 stress cases, semantic equivalence, idempotency.
                                                    Pipeline: 91.3% coverage, 0 lint issues.
Phase 7B: Formatter hardening (close remaining gaps) — NEXT
Phase 8: CLI UX fixes                            — help text + dry-run diff
```

---

## Phase 7B: Formatter Hardening -- Close Remaining Gaps

**Purpose**: Eliminate known risks before release. Five specific gaps remain that could cause data corruption or silent behavior changes in production.

---

### 7B.1: Fix YAML block scalar chomping indicator preservation

**Severity**: HIGH -- semantic data loss
**Problem**: Two separate stripping steps corrupt block scalar semantics:

1. `stripTrailingWhitespace` (printer.go:681): Strips trailing spaces/tabs from EVERY line including block scalar content lines where trailing whitespace may be meaningful.
2. `bytes.TrimRight(out, "\r\n")` (printer.go:65): Unconditionally removes ALL trailing newlines. For `|+` (keep chomping), trailing newlines after the last content line ARE part of the scalar value.

**Root cause**: Both steps operate on the serialized byte stream with no awareness of token boundaries. The tokenizer (`consumeBlockScalar`, tokenizer.go:493) correctly captures the entire block including trailing content. Corruption happens post-serialization.

**Fix -- two changes in `pkg/formatter/yamlfmt/printer.go`:**

**Change 1: Replace `stripTrailingWhitespace(out)` with token-aware serialization.**

New function `serializeWithStrip(tokens []Token) []byte`:
- Accumulates bytes into a line buffer
- When encountering `\n`: trim trailing spaces/tabs from the line buffer, emit line + newline, reset buffer
- When encountering `TokBlockScalar`: flush current line buffer (stripped), then emit the ENTIRE block scalar token raw VERBATIM (no stripping whatsoever), then reset line tracking (block scalar always ends at a line boundary because consumeBlockScalar includes the final newline)
- This replaces both the `buf.Write` loop AND the `stripTrailingWhitespace` call

Implementation detail -- the line buffer approach:
```go
func serializeWithStrip(tokens []Token) []byte {
    var out bytes.Buffer
    var line []byte // accumulates current line content

    for _, tok := range tokens {
        if tok.Kind == TokBlockScalar {
            // Flush line buffer (stripped) before block scalar.
            flushLine(&out, &line)
            // Emit block scalar verbatim -- no whitespace stripping.
            out.Write(tok.Raw)
            continue
        }
        // For all other tokens, accumulate into line buffer.
        for _, b := range tok.Raw {
            if b == '\n' {
                // End of line -- strip trailing whitespace, emit.
                trimmed := bytes.TrimRight(line, " \t")
                out.Write(trimmed)
                out.WriteByte('\n')
                line = line[:0]
            } else if b == '\r' {
                // Handle CR or CRLF
                trimmed := bytes.TrimRight(line, " \t")
                out.Write(trimmed)
                out.WriteByte('\r')
                line = line[:0]
            } else {
                line = append(line, b)
            }
        }
    }
    // Flush remaining (last line without newline).
    if len(line) > 0 {
        trimmed := bytes.TrimRight(line, " \t")
        out.Write(trimmed)
    }
    return out.Bytes()
}

func flushLine(out *bytes.Buffer, line *[]byte) {
    if len(*line) > 0 {
        trimmed := bytes.TrimRight(*line, " \t")
        out.Write(trimmed)
        *line = (*line)[:0]
    }
}
```

**Change 2: Conditional final newline trimming.**

Replace:
```go
out = bytes.TrimRight(out, "\r\n")
if opts.FinalNewline { out = append(out, '\n') }
```

With:
```go
if !endsWithKeepChomping(tokens) {
    out = bytes.TrimRight(out, "\r\n")
}
if opts.FinalNewline && (len(out) == 0 || out[len(out)-1] != '\n') {
    out = append(out, '\n')
}
```

New function `endsWithKeepChomping(tokens []Token) bool`:
```go
func endsWithKeepChomping(tokens []Token) bool {
    // Walk backward to find last non-whitespace token.
    for i := len(tokens) - 1; i >= 0; i-- {
        switch tokens[i].Kind {
        case TokNewline, TokIndent, TokSpace:
            continue
        case TokBlockScalar:
            // Check header for '+' chomping indicator.
            // Header is everything before the first \n in Raw.
            raw := tokens[i].Raw
            for j := 0; j < len(raw); j++ {
                if raw[j] == '\n' || raw[j] == '\r' {
                    break
                }
                if raw[j] == '+' {
                    return true
                }
            }
            return false
        default:
            return false
        }
    }
    return false
}
```

**Delete**: Remove `stripTrailingWhitespace` function (line 681-715) -- it's replaced by `serializeWithStrip`.

**Test cases to add to `stressCorpus` in stress_format_test.go:**

1. Keep chomping (`|+`) with trailing newlines:
   ```
   input: "keep: |+\n    content\n\n\n"
   ```
   Verify: `yaml.Unmarshal` gives value `"content\n\n\n"`

2. Strip chomping (`|-`):
   ```
   input: "strip: |-\n    content\n"
   ```
   Verify: value is `"content"` (no trailing newline)

3. Default clip (`|`):
   ```
   input: "clip: |\n    content\n"
   ```
   Verify: value is `"content\n"` (exactly one trailing newline)

4. Trailing spaces in block content (must be preserved):
   ```
   input: "spaces: |\n    line with trailing   \n    clean line\n"
   ```
   Verify: value contains `"line with trailing   \n"`

**Unit test in yaml_test.go:**
```go
func TestBlockScalarChompingPreservation(t *testing.T) {
    // Test all three chomping modes
    cases := []struct {
        name    string
        input   string
        wantVal string
    }{
        {"keep_trailing", "k: |+\n  text\n\n\n", "text\n\n\n"},
        {"strip_trailing", "k: |-\n  text\n", "text"},
        {"clip_default", "k: |\n  text\n", "text\n"},
        {"keep_folded", "k: >+\n  text\n\n\n", "text\n\n\n"},
        {"strip_folded", "k: >-\n  text\n", "text"},
    }
    // Format each, then yaml.Unmarshal the formatted output and compare value
}
```

**Files modified**: `pkg/formatter/yamlfmt/printer.go`
**Files added/modified for tests**: `cmd/cfv/stress_format_test.go`, `pkg/formatter/yamlfmt/yaml_test.go`

---

### 7B.2: XML semantic equivalence verification

**Severity**: MEDIUM -- untested assertion
**Problem**: `xmlEquivalent` in `stress_format_test.go` is a no-op. The 8 XML stress tests pass without verifying the DOM is unchanged.

**Implementation in `cmd/cfv/stress_format_test.go`:**

Add types:
```go
type xmlNode struct {
    Name     string      // "namespace:local" or just "local"
    Attrs    []string    // sorted ["name=value", ...] for deterministic comparison
    Text     string      // concatenated, whitespace-trimmed text content
    Children []*xmlNode
}
```

Add `parseXMLTree(t *testing.T, data []byte) *xmlNode`:
- Create `xml.NewDecoder(bytes.NewReader(data))`
- Set `decoder.Strict = false` (handle real-world XML)
- Walk tokens:
  - `xml.StartElement`: push new `xmlNode` onto stack, set name from element, sort and store attrs as `"key=value"` strings
  - `xml.EndElement`: pop from stack, append to parent's children
  - `xml.CharData`: trim whitespace; if non-empty, append to current node's Text field
  - `xml.Comment`, `xml.ProcInst`, `xml.Directive`: skip (not semantic)
- Return the root node (or a wrapper if multiple top-level elements exist, though XML requires single root)

Add `requireXMLTreeEqual(t *testing.T, a, b *xmlNode, path string)`:
- Compare `a.Name == b.Name` (fail with path context)
- Compare `a.Attrs` (sorted slices, `require.Equal`)
- Compare `a.Text` (trimmed, normalized whitespace)
- Compare `len(a.Children) == len(b.Children)`
- Recurse for each child pair with path `parent/child[i]`

Replace current `xmlEquivalent`:
```go
func xmlEquivalent(t *testing.T, original, formatted []byte) {
    t.Helper()
    origTree := parseXMLTree(t, original)
    fmtTree := parseXMLTree(t, formatted)
    requireXMLTreeEqual(t, origTree, fmtTree, "root")
}
```

**Import needed**: `"encoding/xml"` (already available in the module)

**Edge cases handled by design**:
- CDATA normalized to CharData by xml.Decoder (automatic)
- Self-closing vs `<tag></tag>` both parse as StartElement + EndElement (automatic)
- Attribute order: we sort attrs before comparing (order-independent)
- Insignificant whitespace: trimmed and skipped if empty
- Namespace prefixes: xml.Decoder resolves them; we use `Space:Local` format

**Files**: `cmd/cfv/stress_format_test.go` only (test infrastructure, no production code)

---

### 7B.3: Fuzz all option combinations

**Severity**: MEDIUM -- untested code paths
**Problem**: Only YAML has `FuzzYAMLFormatterWithOptions`. All other formatters fuzz with default options only.

**Implementation pattern** (same for each formatter):

```go
func FuzzFormatWithOptions(f *testing.F) {
    f.Add([]byte("seed content"), byte(0))
    // ... more seeds ...

    fmtr := Formatter{}
    f.Fuzz(func(t *testing.T, data []byte, optByte byte) {
        opts := DefaultOptions()
        if optByte&0x01 != 0 { opts.SortKeys = true }
        if optByte&0x02 != 0 { opts.IndentWidth = 4 }
        if optByte&0x04 != 0 { opts.FinalNewline = false }
        // format-specific additions below

        result, err := fmtr.Format(data, opts)
        if err != nil { return }

        result2, err := fmtr.Format(result, opts)
        if err != nil {
            t.Fatalf("second format failed: %v\nfirst: %q", err, result)
        }
        if string(result) != string(result2) {
            t.Fatalf("not idempotent:\ninput:  %q\nfirst:  %q\nsecond: %q", data, result, result2)
        }
    })
}
```

**Per-formatter specifics:**

| File | Seeds | Bit 0 | Bit 1 | Bit 2 | Bit 3 | Bit 4 |
|------|-------|-------|-------|-------|-------|-------|
| `tomlfmt/toml_test.go` | `[package]\nname="x"\n`, multiline string | SortKeys | IndentWidth=4 | FinalNewline=false | -- | -- |
| `jsoncfmt/jsonc_test.go` | `{"a":1}`, comments+trailing comma | IndentWidth=4 | FinalNewline=false | -- | -- | -- |
| `propfmt/properties_test.go` | `k=v\n`, continuation, escapes | SortKeys | FinalNewline=false | -- | -- | -- |
| `inifmt/ini_test.go` | `[s]\nk=v\n`, multiple sections | SortKeys | IndentWidth=4 | FinalNewline=false | -- | -- |
| `xmlfmt/xml_test.go` | `<?xml...?><r><c/></r>`, nested | IndentWidth=4 | FinalNewline=false | XMLSelfClosingSpace | -- | -- |
| `yamlfmt/yaml_test.go` (extend existing) | keep existing seeds | SortKeys | QuoteDouble | IndentWidth=4 | FinalNewline=false | QuoteSingle |

**Extend `FuzzYAMLFormatterWithOptions`**: Currently hardcodes `SortKeys=true, QuoteDouble`. Change to use `optByte` parameter to cycle. Add `byte(0)` to existing seed adds.

**Run procedure**: After implementing, run each:
```
go test -fuzz=FuzzFormatWithOptions -fuzztime=45s ./pkg/formatter/tomlfmt/
go test -fuzz=FuzzFormatWithOptions -fuzztime=45s ./pkg/formatter/jsoncfmt/
go test -fuzz=FuzzFormatWithOptions -fuzztime=45s ./pkg/formatter/propfmt/
go test -fuzz=FuzzFormatWithOptions -fuzztime=45s ./pkg/formatter/inifmt/
go test -fuzz=FuzzFormatWithOptions -fuzztime=45s ./pkg/formatter/xmlfmt/
go test -fuzz=FuzzYAMLFormatterWithOptions -fuzztime=45s ./pkg/formatter/yamlfmt/
```

Any failure: save corpus entry, write fix, re-fuzz.

**Files**: 6 test files modified

---

### 7B.4: propfmt EOF continuation -- verify all line ending variants

**Severity**: LOW -- narrowly tested fix
**Problem**: Today's EOF break fix was exercised by one corpus entry. Need explicit coverage.

**Implementation in `pkg/formatter/propfmt/properties_test.go`:**

```go
func TestContinuationAtEOF(t *testing.T) {
    t.Parallel()
    fmtr := propfmt.Formatter{}
    opts := propfmt.DefaultOptions()

    cases := []struct {
        name  string
        input string
    }{
        {"odd_backslash_eof_no_newline", "key = value\\"},
        {"odd_backslash_before_bare_CR", "key = value\\\r"},
        {"odd_backslash_before_CRLF", "key = value\\\r\n"},
        {"even_backslashes_no_continuation", "key = value\\\\"},
        {"triple_backslash_eof", "key = value\\\\\\"},
        {"continuation_then_eof_empty", "key = \\\n"},
        {"continuation_before_bare_CR_content", "key = val\\\ranother = x"},
        {"multi_continuation_then_eof", "key = a\\\n  b\\\n  c\\"},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            result, err := fmtr.Format([]byte(tc.input), opts)
            require.NoError(t, err, "format failed for input %q", tc.input)

            result2, err := fmtr.Format(result, opts)
            require.NoError(t, err, "re-format failed")
            require.Equal(t, string(result), string(result2),
                "not idempotent for %q", tc.input)
        })
    }
}
```

**Additional stress case in `stress_format_test.go`:**
```go
{
    name:             "properties/continuation_at_eof",
    format:           "properties",
    formatter:        propfmt.Formatter{},
    checkEquivalence: propertiesEquivalent,
    input:            "key1 = value\\\n    continued\nkey2 = end\\",
},
```

Note: `propertiesEquivalent` will parse both with `magiconair/properties` which handles continuations correctly. The trailing `\` at EOF in `key2` means the value is `end` (backslash with nothing after it is treated as literal by the library? Need to verify). If the library chokes, wrap in error handling.

**Files**: `pkg/formatter/propfmt/properties_test.go`, `cmd/cfv/stress_format_test.go`

---

### 7B.5: Real-world corpus testing

**Severity**: LOW -- confidence building
**Problem**: All inputs are synthetic.

**Implementation in `cmd/cfv/stress_format_test.go`:**

```go
func TestRealWorldCorpus(t *testing.T) {
    t.Parallel()
    repoRoot := findRepoRoot(t)

    cases := []struct {
        path      string
        format    string
        fmtr      formatter.Formatter
        checkEq   func(t *testing.T, original, formatted []byte)
    }{
        // YAML -- 7 files
        {".golangci.yaml", "yaml", yamlfmt.Formatter{}, yamlEquivalent},
        {".mega-linter.yml", "yaml", yamlfmt.Formatter{}, yamlEquivalent},
        {".pre-commit-hooks.yaml", "yaml", yamlfmt.Formatter{}, yamlEquivalent},
        {"demo.yml", "yaml", yamlfmt.Formatter{}, yamlEquivalent},
        {".github/workflows/go.yml", "yaml", yamlfmt.Formatter{}, yamlEquivalent},
        {".github/workflows/release.yml", "yaml", yamlfmt.Formatter{}, yamlEquivalent},
        {".github/dependabot.yml", "yaml", yamlfmt.Formatter{}, yamlEquivalent},
        // JSON -- 3 files
        {"website/package.json", "json", jsonfmt.Formatter{}, jsonEquivalent},
        {"pkg/configfile/schema.json", "json", jsonfmt.Formatter{}, jsonEquivalent},
        {".markdownlint.json", "json", jsonfmt.Formatter{}, jsonEquivalent},
        // JSONC -- 1 file
        {"website/tsconfig.json", "jsonc", jsoncfmt.Formatter{}, jsoncEquivalent},
    }

    for _, tc := range cases {
        t.Run(filepath.Base(tc.path), func(t *testing.T) {
            t.Parallel()
            src, err := os.ReadFile(filepath.Join(repoRoot, tc.path))
            if os.IsNotExist(err) {
                t.Skipf("file not found: %s (may not exist in all checkouts)", tc.path)
            }
            require.NoError(t, err)

            opts := defaultOpts(tc.format)
            formatted, err := tc.fmtr.Format(src, opts)
            require.NoError(t, err, "format %s", tc.path)

            // Idempotent
            result2, err := tc.fmtr.Format(formatted, opts)
            require.NoError(t, err)
            require.Equal(t, string(formatted), string(result2),
                "not idempotent: %s", tc.path)

            // Semantic equivalence
            tc.checkEq(t, src, formatted)
        })
    }
}

func findRepoRoot(t *testing.T) string {
    t.Helper()
    dir, err := os.Getwd()
    require.NoError(t, err)
    for {
        if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
            return dir
        }
        parent := filepath.Dir(dir)
        if parent == dir {
            t.Fatal("could not find repo root (no go.mod found)")
        }
        dir = parent
    }
}
```

**Why these files**: They're guaranteed to exist in any checkout of this repo. They're real configs maintained by the team. If the formatter breaks them, that's a release blocker.

**Note on `website/tsconfig.json`**: This is actually JSONC (tsconfig allows comments). The JSONC formatter handles it. If it's strict JSON with no comments, `jsoncEquivalent` still works (JSONC is a superset).

**Files**: `cmd/cfv/stress_format_test.go`

---

### Tasks (execution order)

- [ ] 7B.1.1: Implement `serializeWithStrip(tokens []Token) []byte` in printer.go
- [ ] 7B.1.2: Implement `endsWithKeepChomping(tokens []Token) bool` in printer.go
- [ ] 7B.1.3: Replace serialize/strip/trim pipeline in `printFormatted` with new functions
- [ ] 7B.1.4: Delete old `stripTrailingWhitespace` function
- [ ] 7B.1.5: Add `TestBlockScalarChompingPreservation` to yaml_test.go (5 cases)
- [ ] 7B.1.6: Add 4 chomping stress test cases to stressCorpus
- [ ] 7B.1.7: Run all YAML tests + fuzz corpus -- zero regressions
- [ ] 7B.1.8: Fuzz 45s -- zero failures

- [ ] 7B.2.1: Add `xmlNode` type to stress_format_test.go
- [ ] 7B.2.2: Implement `parseXMLTree` using encoding/xml Decoder
- [ ] 7B.2.3: Implement `requireXMLTreeEqual` recursive comparison
- [ ] 7B.2.4: Replace `xmlEquivalent` no-op with real implementation
- [ ] 7B.2.5: Run stress tests -- fix any XML failures

- [ ] 7B.3.1: Add `FuzzFormatWithOptions` to tomlfmt/toml_test.go
- [ ] 7B.3.2: Add `FuzzFormatWithOptions` to jsoncfmt/jsonc_test.go
- [ ] 7B.3.3: Add `FuzzFormatWithOptions` to propfmt/properties_test.go
- [ ] 7B.3.4: Add `FuzzFormatWithOptions` to inifmt/ini_test.go
- [ ] 7B.3.5: Add `FuzzFormatWithOptions` to xmlfmt/xml_test.go
- [ ] 7B.3.6: Extend yamlfmt `FuzzYAMLFormatterWithOptions` with optByte parameter
- [ ] 7B.3.7: Run each fuzz 45s -- save corpus entries for failures
- [ ] 7B.3.8: Fix any failures found

- [ ] 7B.4.1: Add `TestContinuationAtEOF` to propfmt/properties_test.go (8 cases)
- [ ] 7B.4.2: Add 1 continuation_at_eof stress case
- [ ] 7B.4.3: Run -- all pass

- [ ] 7B.5.1: Add `TestRealWorldCorpus` to stress_format_test.go (11 files)
- [ ] 7B.5.2: Add `findRepoRoot` helper
- [ ] 7B.5.3: Run -- fix any failures

- [ ] 7B.6: Full pipeline: vet, fmt, lint, build, test, coverage >= 90%

---


## Phase 8: CLI UX Fixes

**Effort**: < 1 hour
**Risk**: Negligible

### 8.1: Fix stale help text

`cfv format --help` currently says "Formats with registered formatters: json" — hardcoded
string from before JSONC/TOML/Properties formatters were added.

**Fix**: Replace with dynamic list built from the FileTypes registry (any FileType where
`Formatter != nil`).

### 8.2: Dry-run diff output

`cfv format .` (no `--fix`) shows `~` for files that need changes but doesn't show what
would change. Users can't preview before committing.

**Fix**: When a file shows `~`, print a unified diff (before vs after) to stdout. Use the
same format as `git diff` — `---`/`+++` headers with `@@ @@` hunks. Standard library
or minimal diff implementation.

### 8.3: Add JSONC to FormatConfig

The `.cfv.toml` `FormatConfig` struct has per-format overrides for JSON, YAML, HCL, TOML,
XML, INI, ENV, Properties — but not JSONC. Add `JSONC *FormatOptions` field.

---

## Phase 7: Ephemeral CLI Stress Test (ALL Formatters)

**Purpose**: Prove the ENTIRE formatting pipeline works end-to-end through the
real CLI binary. Not a unit test — a functional stress test that exercises the
actual user workflow across every supported format.

**Pattern**:
```
1. Create temp directory
2. Generate/place messy config files for ALL formats:
   - .jsonc (VS Code settings, tsconfig, eslintrc with comments)
   - .toml (Cargo.toml, pyproject.toml with inline tables, multiline arrays)
   - .properties (Spring Boot application.properties with continuations, escapes)
   - .json (package.json, messy spacing)
   - .yaml (docker-compose, k8s manifests, anchors)
   - .hcl (Terraform configs)
   - .ini (my.cnf, php.ini with sections and comments)
   - .env (dotenv with quotes, exports, comments)
   - .xml (Maven POM, Spring beans — if helium fixed)
3. Run: cfv format --fix <dir>
4. Run: cfv check <dir>  (all files still valid)
5. Run: cfv format <dir>  (exit 0, no changes — idempotent)
6. Tear down
```

**Files should be intentionally messy**:
- Inconsistent spacing around separators
- Mixed indentation (tabs + spaces)
- Missing/extra blank lines
- Trailing whitespace
- Missing final newlines
- Valid but ugly formatting

**Success criteria**:
- Step 3: all files formatted (✓ on each)
- Step 4: all files pass validation (exit 0)
- Step 5: all files already formatted (exit 0, no ~ indicators)

**Implementation**: Go test in `cmd/cfv/` using testscript or a custom test
that builds the binary and runs it. Must be automated, not manual QA.

Phases 1-3 can be done sequentially. Phase 6 can slot in anywhere. Phase 4 is the largest effort and benefits from patterns established in Phase 2-3. Phase 5 is externally blocked.

---

## Acceptance Criteria (overall)

- Zero `return src, nil` patterns across all formatters
- Zero silent bail-outs — every formatter either formats or returns an explicit error
- Fuzz 45s per formatter, zero failures, all 8+ formatters
- Full pipeline green, coverage ≥ 90%
- SortKeys works for: JSON ✅, JSONC (new), YAML ✅, Properties (new), INI (new), TOML (new)
- SortKeys is safe: YAML skips sorting when anchor/alias dependencies exist
- All existing fixtures produce identical output (no behavior regression)
- XML handles mixed content correctly (no mangled inline text)
- All review findings CRITICAL/HIGH resolved (Phase 9.1-9.4)

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

### 2026-07-14: Code review findings — 2 critical, 3 high, 10 medium, 20 low

Full review in `REVIEW-formatters.md`. Critical issues:
1. XML mixed content detection is architecturally broken (annotations computed but never consumed)
2. YAML SortKeys breaks anchor/alias ordering → produces invalid YAML

Both require plans (Phase 9). High-severity items are straightforward fixes bundled into Phase 9.

---

## Phase 9: Code Review Fixes

**Date**: 2026-07-14
**Source**: Full code review of all CST-based formatters
**Reference**: `REVIEW-formatters.md` for detailed findings with line numbers

### Execution order

```
9.1: YAML SortKeys anchor safety       — CRITICAL, must fix before any SortKeys release
9.2: XML mixed content fix              — CRITICAL, must fix before XML formatter ships
9.3: XML quick fixes (H1, H2)          — HIGH, 10 min total
9.4: TOML comment duplication fix (H3)  — HIGH, 15 min
9.5: Properties \uXXXX decoding (M1)   — MEDIUM, 30 min
9.6: Stale docs + test gaps (M7-M10)   — MEDIUM, 30 min
9.7: Defensive fixes (M2-M6, M8)       — MEDIUM, 45 min
9.8: Test fixture gaps (L10-L12, L17)   — LOW, 1 hour
9.9: Minor cleanups (L1-L9, L13-L20)   — LOW, 1 hour
9.10: Pipeline verification             — REQUIRED after all fixes
```

---

### 9.1: YAML SortKeys — Anchor/Alias Safety

**Severity**: CRITICAL
**File**: `pkg/formatter/yamlfmt/printer.go:292-303`
**Problem**: `sortKeysAtDepth` sorts purely by key name. If an anchor (`&name`) sorts after its alias (`*name`), output is invalid YAML.

**Approach**: Conservative — detect and skip.

1. Before sorting a set of entries, scan each entry's tokens for `TokAnchor` and `TokAlias` tokens.
2. Build a dependency set: for each alias `*foo`, record that the entry containing `*foo` depends on the entry containing `&foo`.
3. If ANY cross-entry dependency exists in the current scope, skip sorting that scope entirely. Still recurse into children (nested mappings may be safe to sort independently).
4. Add a comment documenting the limitation: "Mappings with anchor/alias dependencies are not sorted to prevent producing invalid YAML."

**Why conservative over topological**: Topological sort adds complexity (cycle detection for recursive anchors, partial ordering), and the use case (sorting keys in a file with anchors) is rare. Users who use anchors typically care about structure over alphabetical order anyway.

**Test strategy**:
- New fixture: `sort_anchors.input.yaml` / `sort_anchors.expected.yaml` — entries with anchors/aliases remain unsorted
- New fixture: `sort_anchors_nested.input.yaml` — anchor at top level, nested mappings still sort
- Verify: existing `sort_keys.expected.yaml` unchanged (no anchors in that fixture)
- Fuzz: verify idempotency still holds

**Files**:
- `pkg/formatter/yamlfmt/printer.go` — add anchor detection in `sortKeysAtDepth`
- `pkg/formatter/yamlfmt/yaml_test.go` — new fixtures
- `pkg/formatter/yamlfmt/testdata/sort_anchors.*`

**Tasks**:
- [x] 9.1.1: Implement `hasAnchorAliasDependency(tokens []Token, entries []mappingEntry) bool`
- [x] 9.1.2: Guard `sortKeysAtDepth` — skip sort if dependencies detected, still recurse
- [x] 9.1.3: Add test fixtures and verify
- [x] 9.1.4: Fuzz 45s, zero failures (pre-existing 5ac1be7e unrelated, tracked for 9.7.1)

**Note**: Fuzz discovered a PRE-EXISTING idempotency bug (not caused by anchor fix):
Input `"x:\n -\nA: \n  0:"` — sort reorders entries, reindent computes different depths
on first vs second format. Root cause: the reorder changes token positions, and
`groupEntries(tokens, 0, len(tokens), targetDepth)` rescans from 0 (see M2/9.7.1).
This will be fixed by 9.7.1 (fix the rescan range). Corpus entry saved as
`testdata/fuzz/FuzzYAMLFormatterWithOptions/5ac1be7eecf0b9a5`.

---

### 9.2: XML Mixed Content Fix

**Severity**: CRITICAL
**Files**: `pkg/formatter/xmlfmt/printer.go:68-143, 153-210`
**Problem**: Two independent failures:
1. `annotate()` only sets Depth on TokIndent — tag tokens retain Depth=-1, so `detectAndMarkMixedContent` can never correctly identify children
2. `insertFormattingWhitespace` (ignore mode) strips all annotations via `removeInsignificantWhitespace`, then rebuilds formatting without consulting `Structural` flags

**Approach**: Inline mixed content detection during formatting (not as a separate annotation pass).

1. **Delete** the current `detectAndMarkMixedContent` function — it's dead code.
2. **In `insertFormattingWhitespace`**: after `removeInsignificantWhitespace`, before inserting newlines/indents, identify mixed-content elements:
   - Walk the cleaned token stream
   - For each `TokOpenTag`, scan forward to its matching `TokCloseTag` (use a depth counter)
   - Between open and close, check if BOTH non-whitespace `TokText` AND element tokens (`TokOpenTag`/`TokSelfClose`) exist
   - If yes: emit all children of that element inline (no newlines, no indent) — preserve their relative order
   - If no: apply normal newline+indent formatting
3. **Remove** the `Structural` field from Token (or repurpose it) — it's currently unused effectively.

**Why inline detection over annotation**: The annotation approach requires propagating information from one pass to another. Since `insertFormattingWhitespace` already rebuilds the token stream, it's cleaner to make the mixed-content decision at the point where formatting is applied.

**Test strategy**:
- New fixture: `mixed_content.input.xml` — `<p>Hello <b>world</b>!</p>` must stay inline
- New fixture: `mixed_deep.input.xml` — mixed content nested inside formatted elements
- Existing fixtures must not change (no existing fixture has mixed content)
- Fuzz: idempotency still holds

**Files**:
- `pkg/formatter/xmlfmt/printer.go` — rewrite mixed content handling
- `pkg/formatter/xmlfmt/xml_test.go` or `testdata/` — new fixtures

**Tasks**:
- [x] 9.2.1: Delete `detectAndMarkMixedContent` and `Structural` field
- [x] 9.2.2: Implement inline mixed content detection in `insertFormattingWhitespace`
- [x] 9.2.3: Add test fixtures (mixed_content, mixed_deep)
- [x] 9.2.4: Verify all existing fixtures unchanged
- [x] 9.2.5: Fuzz 45s, zero failures (2.37M executions)

---

### 9.3: XML Quick Fixes

**Severity**: HIGH
**Effort**: 10 minutes total

**9.3.1: Fix `<?xml-stylesheet?>` misclassification**

File: `xmlfmt/tokenizer.go:110`

Current:
```go
} else if t.startsWith("<?xml") || t.startsWith("<?XML") {
    t.consumeXMLDecl(start)
```

Fix — check that the character after `<?xml` is whitespace or `?`:
```go
} else if (t.startsWith("<?xml") || t.startsWith("<?XML")) &&
    t.pos+5 < len(t.src) && (t.src[t.pos+5] == ' ' || t.src[t.pos+5] == '\t' ||
    t.src[t.pos+5] == '?' || t.src[t.pos+5] == '\n' || t.src[t.pos+5] == '\r') {
    t.consumeXMLDecl(start)
```

**9.3.2: Fix double newline after XMLDecl/Doctype**

File: `xmlfmt/printer.go:194-199`

Remove the explicit `\n` insertion from the `TokXMLDecl`/`TokDoctype` case. The following token's `needsNewlineBefore` handler already inserts the newline.

**Tasks**:
- [x] 9.3.1: Fix PI classification in tokenizer.go
- [x] 9.3.2: Fix double newline in printer.go (done as part of 9.2)
- [x] 9.3.3: Add test: `<?xml-stylesheet type="text/xsl" href="style.xsl"?>` tokenizes as TokProcInst
- [x] 9.3.4: Add test: XMLDecl followed by root element produces exactly one newline between them

---

### 9.4: TOML Comment Duplication in Arrays

**Severity**: HIGH
**File**: `pkg/formatter/tomlfmt/printer.go:288-324`
**Problem**: `printArrayMultiline` writes comments on their own lines, then calls `writeValueTokensTrimmed(elem)` which writes ALL non-whitespace/newline tokens including the comments again.

**Fix**: Filter Comment tokens from the element slice before passing to `writeValueTokensTrimmed`:

```go
// In printArrayMultiline, replace the default case:
default:
    p.buf.WriteString(elemIndent)
    // Filter out comment tokens — already emitted above.
    valueTokens := make([]Token, 0, len(elem))
    for _, t := range elem {
        if t.Kind != Comment && t.Kind != Newline && t.Kind != Whitespace {
            valueTokens = append(valueTokens, t)
        }
    }
    p.writeValueTokensTrimmed(valueTokens)
    p.buf.WriteByte(',')
    goto nextElem
```

**Test strategy**:
- New fixture: `array_with_comments.input.toml` / `.expected.toml`:
  ```toml
  deps = [
    # web framework
    "actix-web",
    # serialization
    "serde",
  ]
  ```
- Verify output has each comment once, not duplicated

**Tasks**:
- [x] 9.4.1: Fix `printArrayMultiline` to filter comments before `writeValueTokensTrimmed`
- [x] 9.4.2: Add fixture and verify
- [x] 9.4.3: Run existing TOML tests — no regressions

---

### 9.5: Properties `\uXXXX` Decoding in SortKeys

**Severity**: MEDIUM
**File**: `pkg/formatter/propfmt/printer.go:106-116`
**Problem**: `decodeKey` strips backslashes but doesn't decode `\uXXXX` → keys with unicode escapes sort wrong.

**Fix**: Add unicode escape decoding:
```go
func decodeKey(raw []byte) string {
    var b strings.Builder
    for i := 0; i < len(raw); i++ {
        if raw[i] == '\\' && i+1 < len(raw) {
            i++
            if raw[i] == 'u' && i+4 < len(raw) {
                hex := string(raw[i+1 : i+5])
                if r, err := strconv.ParseUint(hex, 16, 32); err == nil {
                    b.WriteRune(rune(r))
                    i += 4
                    continue
                }
            }
            b.WriteByte(raw[i])
        } else {
            b.WriteByte(raw[i])
        }
    }
    return b.String()
}
```

**Test strategy**:
- New fixture: `unicode_sort.input.properties` with `\u0042anana` and `Apple` — should sort as Apple, Banana
- Verify existing fixtures unchanged

**Tasks**:
- [x] 9.5.1: Implement `\uXXXX` decoding in `decodeKey`
- [x] 9.5.2: Add fixture and test
- [x] 9.5.3: Verify no regressions

---

### 9.6: Documentation and Test Gap Fixes

**Severity**: MEDIUM
**Effort**: 30 minutes

**Tasks**:
- [x] 9.6.1: Fix INI lexer comments (lexer.go:82 "through closing ]" → "to end of line"; lexer.go:125 key+space comment)
- [x] 9.6.2: Fix XML stale package docs (xml.go:1-10, 45) — update to reflect CST architecture, remove helium serialization claims, remove ErrSkipped mixed content claim
- [x] 9.6.3: Delete orphaned comment at xml.go:72
- [x] 9.6.4: Add XML preserve-mode fixture pair (`.opts.json` with `XMLWhitespaceSensitivity: "preserve"`)
- [x] 9.6.5: Add XML self-closing space fixture (`.opts.json` with `XMLSelfClosingSpace: true`)
- [x] 9.6.6: Document JSONC inline-array trailing comma removal (comment in jsonc.go:160)

---

### 9.7: Defensive and Performance Fixes

**Severity**: MEDIUM
**Effort**: 45 minutes

**Tasks**:
- [ ] 9.7.1: YAML — fix `sortKeysAtDepth` rescan: `groupEntries(tokens, from, to, targetDepth)` after reorder (printer.go:298)
- [ ] 9.7.2: YAML — eliminate double parse: replace validation Decoder loop + separate Node unmarshal with single `yaml.Node` unmarshal that serves both purposes (yaml.go:69 + printer.go:113)
- [ ] 9.7.3: TOML — change `containsByte` to take `[]byte` and use `bytes.IndexByte` (tokenizer.go:318)
- [ ] 9.7.4: JSONC — fix double type assertion in `isInlineArray` (jsonc.go:207): capture result from first `ok` check
- [ ] 9.7.5: JSONC — add defensive comment to `sortObject` type assertion (jsonc.go:285)
- [ ] 9.7.6: Shared — add `TestAllExpectedFormattersRegistered` test in `pkg/filetype/`
- [ ] 9.7.7: Shared — fix swallowed `json.Unmarshal` error in `fixture_opts.go` (return error or accept `testing.TB`)

---

### 9.8: Test Fixture Gaps

**Severity**: LOW
**Effort**: 1 hour

**Tasks**:
- [ ] 9.8.1: TOML — add `array_tables.input.toml` / `.expected.toml` (tests `[[entries]]`)
- [ ] 9.8.2: Properties — add fixtures: empty file, comments-only, key-with-no-value, unicode escapes, `!`-style comments
- [ ] 9.8.3: INI — add fixtures: default section (keys before first header), duplicate sections, empty sections
- [ ] 9.8.4: JSONC — add fixture with tab indent and one with IndentWidth=4
- [ ] 9.8.5: XML — add token-kind assertion test (verify `<?xml-stylesheet?>` → TokProcInst, not TokXMLDecl)

---

### 9.9: Minor Cleanups

**Severity**: LOW
**Effort**: 1 hour (can be done incrementally)

**Tasks**:
- [ ] 9.9.1: TOML — remove debug trace tests (tokenizer_test.go:335-418) or add assertions
- [ ] 9.9.2: TOML — pre-allocate token slice in tokenizer (`make([]Token, 0, len(src)/4)`)
- [ ] 9.9.3: TOML — pre-compute double-newline bytes in `ensureBlankLine` (avoid `append(nl, nl...)`)
- [ ] 9.9.4: YAML — guard empty lines in `shiftBlockScalarIndent` (skip indent for empty content lines)
- [ ] 9.9.5: YAML — align `isBlockScalarStart` to reject digit '0' (match `consumeBlockScalar`)
- [ ] 9.9.6: JSONC — add comment to `reindentExtra` explaining blank-line collapsing is intentional
- [ ] 9.9.7: JSONC — fix `isInlineArray` totalLen off-by-2 or add comment explaining conservative bias
- [ ] 9.9.8: INI — add comment to `valueStartsWithQuote` explaining ini.v1 PreserveSurroundedQuote behavior
- [ ] 9.9.9: Shared — investigate `Result` struct usage; remove if dead code
- [ ] 9.9.10: Shared — add comment to `formatters.go` documenting init() ordering dependency

---

### 9.10: Pipeline Verification

**Tasks**:
- [ ] 9.10.1: `go vet ./...`
- [ ] 9.10.2: `gofmt -s -l -e .` (no output)
- [ ] 9.10.3: `golangci-lint run ./...` (zero findings)
- [ ] 9.10.4: `go build -o /dev/null cmd/validator/validator.go`
- [ ] 9.10.5: `go test -cover -coverprofile coverage.out ./...` (all pass)
- [ ] 9.10.6: Coverage ≥ 90%
- [ ] 9.10.7: Fuzz 45s per formatter (YAML, XML, TOML, JSONC, Properties, INI)

---

### 9.11: YAML QuoteStyle Idempotency Bug (Pre-existing)

**Severity**: MEDIUM (correctness — non-idempotent output)
**File**: `pkg/formatter/yamlfmt/printer.go` — `applyQuoteStyle` / `convertQuote`
**Discovered**: Fuzz corpus `008bea5e6f114ffc`
**Problem**: Quote style conversion is non-idempotent when value tokens have trailing whitespace.

**Root cause (traced step by step):**

Input: `A: ''\t` (key `A`, value `''` with trailing tab)

Pass 1:
1. Tokenizer → `TokValue("''\t")` (trailing tab is part of the value raw)
2. `applyQuoteStyle` checks: `raw[0]='` and `raw[len-1]='\t'` → last byte is NOT `'` → **skip** (no conversion)
3. Serialized: `A: ''\t\n`
4. `stripTrailingWhitespace` removes the `\t` → output: `A: ''\n`

Pass 2:
1. Tokenizer → `TokValue("''")` (clean, no trailing tab)
2. `applyQuoteStyle` checks: `raw[0]='` and `raw[len-1]='` → MATCHES → converts `''` to `""`
3. Output: `A: ""\n`

**The disconnect:** `applyQuoteStyle` operates on token `Raw` which may include trailing whitespace. `stripTrailingWhitespace` operates on the serialized byte buffer post-serialization. These two steps see different content for the same logical value.

**Fix: Trim trailing whitespace in `applyQuoteStyle` at the detection site.**

This is the minimal correct fix — trim when checking quote boundaries, and pass the trimmed value to `convertQuote`. The trailing whitespace is dropped from the output (which `stripTrailingWhitespace` would do anyway), and the quote detection sees the same content on every pass.

```go
func applyQuoteStyle(tokens []Token, style formatter.QuoteStyle) {
    for i := range tokens {
        if tokens[i].Kind != TokValue {
            continue
        }
        raw := bytes.TrimRight(tokens[i].Raw, " \t") // trim for detection
        if len(raw) < 2 {
            continue
        }
        first, last := raw[0], raw[len(raw)-1]
        if first == '"' && last == '"' {
            tokens[i].Raw = convertQuote(raw, style, '"')
        } else if first == '\'' && last == '\'' {
            tokens[i].Raw = convertQuote(raw, style, '\'')
        }
    }
}
```

**Why not a separate trim pass (option rejected):** Adding a loop that trims all TokValue tokens before quote conversion works but is heavier than needed. The trim only matters for quote detection — no other code path is affected by trailing whitespace on value tokens (it gets stripped in the final output regardless).

**Why not fixing the tokenizer (option rejected):** The tokenizer would need to understand quoting structure to know where the value ends and trailing whitespace begins. That violates the "values are opaque" design principle.

**Edge cases to verify:**
- Value with trailing spaces: `key: value   ` → trimmed to `key: value`, quote check works
- Quoted value with trailing tab: `key: 'val'\t` → trimmed to `key: 'val'`, conversion fires correctly
- Empty quoted value with trailing WS: `key: ''\t` → trimmed to `key: ''`, conversion fires
- Value that IS only whitespace: `key:  \t` → trimmed to `key:` (empty) — should be fine
- Multi-word unquoted value: `key: hello world` → no trailing WS, unchanged

**Test strategy:**
- Fuzz corpus `008bea5e6f114ffc` must pass (idempotent output)
- All existing fixtures must produce identical output
- Run fuzz 45s with QuoteStyle options to find any additional edge cases
- Add explicit test case for trailing-whitespace-with-quotes scenario

**Tasks:**
- [ ] 9.11.1: Add trailing whitespace trimming for TokValue in `printFormatted` (after annotate, before quote style)
- [ ] 9.11.2: Verify fuzz corpus `008bea5e6f114ffc` passes
- [ ] 9.11.3: Run all fixtures — zero regressions
- [ ] 9.11.4: Add explicit test case for the scenario
- [ ] 9.11.5: Fuzz 45s with QuoteStyle options, zero new failures

---

### 2026-07-14: Custom YAML tokenizer over goccy/go-yaml AST

- goccy/go-yaml's `file.String()` is a **reconstructive serializer**, not a CST printer. It regenerates output from Position fields using `strings.Repeat(" ", column-1)` and hardcoded formatting patterns. It does NOT walk token Origins.
- Instability in `file.String()`: flow mappings inside sequences produce extra padding that changes on re-serialization. This is a library bug we can't fix.
- goccy's token `Origin` field preserves original text, but whitespace boundaries are inconsistent — indent may be on the preceding token's tail OR the following token's head. No clean rule.
- google/yamlfmt uses same decode/encode pattern (go-yaml/yaml v3 fork) with placeholder-comment hacks (`#magic___^_^___line`) for blank-line preservation. Same architectural flaw.
- prettier's YAML plugin uses `yaml-unist-parser` (unist AST) and slices `originalText` for verbatim content. Closer to correct but still relies on an IR engine for indentation.
- **No Go library provides a lossless YAML token stream suitable for formatting.** Confirmed by evaluating goccy, go-yaml, yamlfmt.
- Decision: write a format-only tokenizer. Same architecture as TOML (tokenize → modify indent → print). Classify bytes into tokens where IndentToken is the only token that gets modified. Block scalars and flow collections are opaque.
- Reference: prettier for expected output behavior. Our output should be structurally equivalent to prettier on real-world config YAML.
- This is the first pure CST YAML formatter in Go. No one else has done it because YAML is hard. We're doing it because we're the one config file tool and we're doing it right.

## Phase 7C: Fix Fuzz-Found Bugs

**Date**: 2026-07-15
**Source**: Phase 7B.3 fuzz testing with option combinations

---

### 7C.1: YAML reindent breaks sequence mapping structure with IndentWidth != 2

**Severity**: HIGH -- affects any user with `indent_width = 4` and sequence mappings
**Input**: Any YAML with sequence entries containing mappings:
```yaml
items:
  - name: first
    value: 1
```
**With IndentWidth=4, our formatter produces**:
```yaml
items:
    - name: first
        value: 1       # WRONG: 8 spaces (depth 2 × 4)
```
**Prettier and yamlfmt produce**:
```yaml
items:
    - name: first
      value: 1         # CORRECT: 6 spaces (4 for dash + 2 for "- " offset)
```

**Root cause**: `reindentTokens` (printer.go:196) computes `newIndent = depth * targetWidth` for ALL structural tokens. This is wrong for sequence item children. In YAML, the children of a `- ` are at `dash_indent + 2` (the width of `"- "`), not at `(depth+1) * targetWidth`.

The depth model doesn't distinguish between:
- A mapping key indented because it's nested in another mapping (multiply by width)
- A mapping key indented because it's inside a sequence item (offset by 2 from dash)

**How prettier handles it**: Prettier uses a separate "indent type" concept. When entering a sequence item, the indent increases by 2 (the `- ` width). When entering a nested mapping, the indent increases by `tabWidth`. These are different operations.

**Fix approach -- detect dash context during reindent**:

In `reindentTokens`, when computing `newIndent` for a structural token, check if it's a child of a sequence item (i.e., preceded by a `TokDash` at the same or parent scope). If so, the indent should be `dash_indent + 2`, not `depth * targetWidth`.

Specifically:
1. Track whether each depth level was entered via a dash or via a mapping key.
2. Compute indent differently:
   - Mapping nesting: `parent_indent + targetWidth`
   - Dash nesting: `dash_position + 2` (where dash_position = the indent of the `-` token)

Implementation:

The `computeDepths` function already builds a stack. Extend it to also record whether each level was entered via a dash (has a `TokDash` at that indent). Then in `reindentTokens`, use this information:

```go
func reindentTokens(tokens []Token, targetWidth int) {
    // Build indent plan: for each depth, what should the actual indent be?
    // depth 0: 0
    // depth 1 (from mapping): targetWidth
    // depth 1 (from dash): parentIndent + targetWidth (for the dash line)
    //   children of dash: dashIndent + 2
    // depth 2 (from mapping under dash): dashIndent + 2 + targetWidth
    // etc.
}
```

Actually, the simplest correct approach: **compute indent based on the parent token type**, not just depth.

Alternative simpler approach: **Look backward from each structural indent token to find if it's preceded (on a prior line) by a dash at a shallower indent.** If the token's indent in the original file is `dash_original_indent + 2`, then in the new file it should be `dash_new_indent + 2`.

Simplest correct fix:

Add a `DashChild bool` flag to the token metadata. During `computeDepths`, if a structural indent's immediate parent in the indent stack was introduced by a line containing a `TokDash`, mark it as `DashChild = true`.

Then in `reindentTokens`:
```go
if tokens[i].Structural && tokens[i].Depth >= 0 {
    if tokens[i].DashChild {
        // Find the dash's new indent (previous depth's indent + targetWidth for the dash itself)
        // then add 2 for "- " offset.
        parentDepth := tokens[i].Depth - 1
        dashIndent := parentDepth * targetWidth  // simplified -- need the actual dash indent
        newIndent = dashIndent + targetWidth + 2  // dash at its depth + 2 for "- "
        // Wait, this isn't right either...
    } else {
        newIndent = tokens[i].Depth * targetWidth
    }
}
```

Let me think more carefully. Prettier's model:

```
items:           indent=0, depth=0
    -            indent=4, depth=1 (items is a sequence, child indent = 0 + tabWidth)
      name: x   indent=6, depth=2 (dash child, indent = 4 + 2)
      value: 1  indent=6, depth=2 (dash child, indent = 4 + 2)
    -            indent=4, depth=1
      name: y   indent=6, depth=2
```

The rule: when a depth level is introduced by a sequence item, the CHILDREN of that item are at `dash_indent + 2`. The dash itself is at the normal depth position.

So the computation is:
- Non-dash structural lines: `depth * targetWidth`
- Lines that are siblings of a dash on the same line (mapping keys after `-`): NOT separately indented, they're on the dash line
- Lines that are children of a dash entry: `dash_indent + 2`

Actually wait -- looking at prettier output again:
```
    - name: first   # dash at indent 4, "name" is on same line as dash
      value: 1      # sibling of "name", at indent 6 (dash_indent + 2)
```

So `name` is at the same indent as the dash (it's ON the dash line, no indent token). `value` is on its own line at `dash_indent + 2 = 6`. Both have the same depth. The reindenter needs to know: "this indent is for a key that's a sibling of the first key on the dash line, so it goes at dash_pos + 2".

**Clearest formulation of the rule**:

For each structural indent token, compute its new indent as:
- If this line is a **continuation of a dash entry** (i.e., the dash was on a line above at a shallower indent, and this line's indent puts it inside that dash's scope): `new_indent = new_dash_indent + 2`
- Otherwise: `new_indent = depth * targetWidth`

How to detect "continuation of a dash entry": Look at the indent stack. If the entry at `depth - 1` in the stack has a dash, then this token is a dash child.

**Refined implementation plan**:

1. In `computeDepths`, when pushing a new level onto the stack, also record whether the TOKEN following this indent on the same line is a `TokDash`. Add a `hasDash` field to the stack level.

2. For each structural indent at depth D, check if the stack level at depth D-1 has `hasDash = true`. If so, this token is a dash-child.

3. In `reindentTokens`, maintain a parallel structure that tracks the new indent for each depth level. When a depth is a dash-level, its children get `indent_at_depth + 2` instead of `(depth) * targetWidth`.

Concretely — I think the cleanest approach is to compute a `targetIndent[]` array indexed by depth:

```go
targetIndent[0] = 0
targetIndent[1] = targetWidth (or parentIndent+2 if parent is a dash)
targetIndent[2] = ...
```

But this is hard because the same depth can be dash-child in one place and non-dash-child in another (different parts of the document).

**Even simpler**: During the reindent pass, maintain a stack of `(depth, newIndent, isDash)`. When processing each indent:
- If depth > stack top: push. Compute newIndent based on whether the TOP of stack is a dash level.
- If depth == stack top: same indent as before (sibling).
- If depth < stack top: pop until we find it.

```go
type indentLevel struct {
    depth     int
    newIndent int
    isDash    bool // whether this level contains a dash on its line
}
```

For the first token at a new deeper depth:
- Check if there's a `TokDash` between the previous `TokNewline` and this indent (look backward)
- Actually, check if the current line (tokens following this indent) starts with `TokDash`

Wait, the issue is: the DASH line itself is fine (`- name: first` at depth 1 gets `1 * targetWidth = 4`). It's the NEXT line (`value: 1` at depth 2) that's wrong. So we need to know: "was the previous line at depth 1 a dash line?"

Let me re-examine: when `computeDepths` processes the indent stack, it assigns depth 2 to `value:` because its indent (4 spaces) is deeper than the dash's indent (2 spaces in original). The dash line's indent gets depth 1.

**Final approach** (simplest that matches prettier):

1. Add a field to the Token: `AfterDash bool` -- set during `computeDepths` when the PREVIOUS stack entry (parent) had a dash on its line.

2. Detect "parent has dash": After assigning depth, scan backward from current indent to find the most recent indent at depth-1. Check if tokens between that indent and its newline include a `TokDash`.

3. In `reindentTokens`: when `tokens[i].AfterDash`, compute indent as:
   ```
   parentNewIndent + 2
   ```
   where `parentNewIndent` is the reindented value of the parent indent (the dash line).

This requires tracking what the parent's new indent was. Use a map or stack: `newIndentByDepth[depth-1]` gives the parent's new indent, then add 2.

```go
func reindentTokens(tokens []Token, targetWidth int) {
    lastDelta := 0
    newIndentAtDepth := make(map[int]int) // depth -> last computed new indent at that depth

    for i := range tokens {
        if tokens[i].Kind != TokIndent { continue }
        oldIndent := len(tokens[i].Raw)
        var newIndent int

        if tokens[i].Structural && tokens[i].Depth >= 0 {
            if tokens[i].AfterDash {
                // Child of a sequence entry: parent's indent + 2
                parentIndent := newIndentAtDepth[tokens[i].Depth-1]
                newIndent = parentIndent + 2
            } else {
                newIndent = tokens[i].Depth * targetWidth
            }
            newIndentAtDepth[tokens[i].Depth] = newIndent
            lastDelta = newIndent - oldIndent
        } else {
            newIndent = oldIndent + lastDelta
            if newIndent < 0 { newIndent = 0 }
        }
        tokens[i].Raw = []byte(strings.Repeat(" ", newIndent))
        // ... block scalar shift as before
    }
}
```

**Test strategy**:
- The fuzz corpus entry: `- 0: 0\n  1:` with IndentWidth=4 must be idempotent
- Normal sequence mappings with IndentWidth=4 must match prettier output
- Deeply nested sequences: `- - - value` must handle multiple dash levels
- Mixed dash and non-dash nesting
- All existing fixtures (IndentWidth=2) must be unchanged (when depth*2 == parentIndent+2, both formulas give same result)

**Files**: `pkg/formatter/yamlfmt/printer.go` (Token struct, computeDepths, reindentTokens)

---

### 7C.2: TOML SortKeys breaks when entries have leading whitespace (IndentWidth > 0)

**Severity**: MEDIUM -- affects users with `sort_keys = true` and any indent setting
**Root cause**: `extractKey` (printer.go:708) breaks on the FIRST `Whitespace` token in the group:
```go
if tok.Kind == Equals || tok.Kind == Whitespace {
    break
}
```

When entries are indented (e.g., 4-space indent under a table header), the group's token list STARTS with a Whitespace token (the indent). `extractKey` immediately breaks, returns empty string. All entries get key `""`, sort is stable on equal keys, but order depends on input order -- non-idempotent.

**Fix**: Skip LEADING whitespace before extracting the key:
```go
func extractKey(group Group) string {
    if group.Kind == GroupComment {
        return ""
    }
    var b strings.Builder
    seenKey := false
    for _, tok := range group.Tokens {
        if tok.Kind == Whitespace && !seenKey {
            continue // skip leading whitespace (indentation)
        }
        if tok.Kind == Equals || (tok.Kind == Whitespace && seenKey) {
            break // stop at separator
        }
        switch tok.Kind {
        case BareKey, BasicString, LiteralString:
            seenKey = true
            _, _ = b.Write(tok.Raw)
        case Dot:
            seenKey = true
            _ = b.WriteByte('.')
        default:
        }
    }
    return b.String()
}
```

**Test strategy**:
- Fuzz corpus entry: `[[0]]\n\nB=""\nA=""` with SortKeys+IndentWidth=4 must be idempotent (A before B on both passes)
- Real-world: `[dependencies]\nserde = "1.0"\nactix-web = "4.0"` with SortKeys+IndentWidth=2 (default) must sort correctly
- Existing TOML fixtures with SortKeys must be unchanged

**Files**: `pkg/formatter/tomlfmt/printer.go` (extractKey only -- one function, ~5 line change)

---

### 7C.3: FinalNewline=false produces invalid output for some formats

**Severity**: LOW -- only with explicitly disabled FinalNewline + pathological input
**Affected**: JSONC (hujson requires trailing newline in some cases), Properties (magiconair/properties requires it for continuation handling), INI (gopkg.in/ini.v1 requires it for certain value patterns)

**Root cause**: The format function strips the final newline, then on second pass the VALIDATION step (which runs before formatting) rejects the output because the underlying parser library needs the newline.

**Behavior of competition**: Prettier does not offer a `--no-final-newline` option. It ALWAYS appends one. EditorConfig's `insert_final_newline = false` is respected by some tools but ignored by format-sensitive ones.

**Options evaluated**:

A. **Remove FinalNewline option entirely** -- Too aggressive. Some users legitimately want it (e.g., concatenation pipelines).

B. **Validate output before returning** -- Add a post-format validation step: after formatting, try to re-parse the output. If it fails, return an error. This catches ALL "formatter produces invalid output" bugs, not just FinalNewline ones. ~10 lines per formatter.
   - Pro: Safety net for ALL formatting bugs, not just this one
   - Con: Performance cost (double-parse), may catch things we don't want to block on

C. **Ignore FinalNewline=false for formats that require it** -- When FinalNewline=false but the format requires a trailing newline for valid output, silently keep the newline.
   - Pro: Never produces invalid output
   - Con: Silently ignores a user setting (violates "no silent bail-outs" principle)

D. **Return error when FinalNewline=false would produce invalid output** -- Try without newline; if re-parse fails, return error explaining the incompatibility.
   - Pro: Explicit, user knows what's wrong
   - Con: Error on a formatting option feels harsh

**Recommendation**: Option B (validate output). It's the safety net that catches not just FinalNewline issues but ANY bug where the formatter produces invalid output. It would have caught bugs #1 and #2 as well. The performance cost is negligible for config files (they're small).

Implementation: After formatting, attempt `validator.ValidateSyntax(output)`. If it fails, return a descriptive error: `"formatter produced invalid output (possible bug or incompatible options): %v"`. This makes the formatter fail-safe -- it NEVER silently returns corrupted data.

**But**: This means the existing fuzz failures would become errors instead of silent corruption. That's BETTER, but it means users who hit these edge cases get an error instead of a formatted file. We should fix the root causes (#1, #2) and then the safety net only catches truly unexpected bugs.

**Implementation plan**:
1. Fix #1 (YAML indent) and #2 (TOML sort key extraction) first
2. Then add the safety-net validation to ALL formatters
3. Re-run fuzz -- the JSONC/Properties/INI failures should now return errors (graceful) instead of silently corrupting

Safety net code (in each formatter's Format function, after the format pipeline):
```go
// Safety net: verify formatted output is still valid.
if _, err := validate(output); err != nil {
    return nil, fmt.Errorf("formatter produced invalid output: %w (input may be incompatible with the requested options)", err)
}
```

Where `validate` is the same validation function used at the top of Format.

**Files**: All 9 formatter files (`*/format.go` or `*/properties.go` etc.)

---

### Tasks

- [ ] 7C.1.1: Add `AfterDash bool` field to Token struct in tokenizer.go
- [ ] 7C.1.2: In `computeDepths`, detect dash-parent and set AfterDash on child indents
- [ ] 7C.1.3: Update `reindentTokens` to use `parentIndent + 2` for AfterDash tokens
- [ ] 7C.1.4: Add fuzz corpus entry back, verify it passes
- [ ] 7C.1.5: Test against prettier output for sequence mappings with IndentWidth=4
- [ ] 7C.1.6: Verify all existing YAML fixtures unchanged (IndentWidth=2 produces same result)
- [ ] 7C.1.7: Fuzz 45s with options, zero new failures

- [ ] 7C.2.1: Fix `extractKey` to skip leading whitespace
- [ ] 7C.2.2: Add fuzz corpus entry back, verify it passes
- [ ] 7C.2.3: Verify existing TOML sort fixtures unchanged
- [ ] 7C.2.4: Fuzz 45s with options, zero new failures

- [ ] 7C.3.1: Add output validation safety net to all 9 formatters
- [ ] 7C.3.2: Re-run fuzz for JSONC/Properties/INI -- verify they return errors (not silent corruption)
- [ ] 7C.3.3: Add fuzz corpus entries back as regression tests (they should pass now -- error is acceptable)
- [ ] 7C.3.4: Verify no performance regression on normal files (benchmark)

- [ ] 7C.4: Full pipeline verification

---


### 7C.3: Replace indent-based depth with AST-derived depth (single source of truth)

**Severity**: Architectural — eliminates the design flaw that caused 7C.1 (AfterDash) and the column-0 sort bug
**Scope**: `pkg/formatter/yamlfmt/printer.go` and `pkg/formatter/yamlfmt/tokenizer.go`

---

#### Problem Statement

The YAML formatter computes structural depth from indent widths (a heuristic). This is wrong because YAML allows value blocks at column 0, making indent != structure. We patched this twice:
- `AfterDash` for reindent (7C.1)
- Was about to add `SortDepth` for sort (the mixed approach)

Both patches treat symptoms. The root cause: we're deriving structure from rendering when we have the authoritative structure (the AST) available.

#### Design Principle

**One source of truth: the yaml.v3 Node tree.** The AST tells us:
- What depth each key is at (mapping nesting level)
- Whether a key is inside a sequence item (dash-relative indent)
- Which keys are siblings (same parent MappingNode)

This replaces `computeDepths`, `AfterDash`, and the indent-width heuristic entirely.

---

#### What the AST Provides (proven by trace)

For `servers:\n  - host: a\n    port: 80\ndb:\n  host: localhost\n`:

```
KEY "servers" L1:C1 depth=0 inSeq=false
  SEQ_ITEM[0] L2 depth=1
  KEY "host" L2:C5 depth=1 inSeq=true
  KEY "port" L3:C5 depth=1 inSeq=true
KEY "db" L6:C1 depth=0 inSeq=false
  KEY "host" L7:C3 depth=1 inSeq=false
```

For reindent with `targetWidth=4`:
- `servers` (depth=0, !inSeq): indent = `0 * 4 = 0`
- `- host` line (depth=1, seq item): indent = `0*4 + 4 = 4` (targetWidth for the dash)
- `port` (depth=1, inSeq=true): indent = `dashIndent + 2 = 4 + 2 = 6`
- `db` (depth=0, !inSeq): indent = `0 * 4 = 0`
- `host` under db (depth=1, !inSeq): indent = `1 * 4 = 4`

For sort: group keys at depth=0 → {servers, db} → sort → {db, servers}

---

#### New Token Fields (replace Depth + AfterDash)

```go
type Token struct {
    Kind       TokenKind
    Raw        []byte
    Structural bool   // whether this indent should be renormalized (from structuralLines)
    Line       int    // 1-based source line number (set during annotate)
    // AST-derived fields (set by assignASTMetadata):
    ASTDepth   int    // mapping nesting depth from root. -1 = not annotated.
    InSeq      bool   // true if this key is inside a sequence item (indent = parent + 2)
}
```

Removed: `Depth int`, `AfterDash bool`

---

#### New Function: `buildASTMetadata`

Walks the yaml.v3 Node tree and produces a map from `line_number → metadata`:

```go
type lineMetadata struct {
    depth int  // mapping nesting depth
    inSeq bool // true if this line's key is inside a sequence entry
}

func buildASTMetadata(src []byte) map[int]lineMetadata {
    if len(src) > 0 && src[len(src)-1] != '\n' {
        src = append(bytes.Clone(src), '\n')
    }
    var root yaml.Node
    if err := yaml.Unmarshal(src, &root); err != nil {
        return nil // fallback to legacy behavior
    }
    meta := make(map[int]lineMetadata)
    collectMetadata(&root, meta, 0, false)
    return meta
}

func collectMetadata(n *yaml.Node, meta map[int]lineMetadata, depth int, inSeq bool) {
    switch n.Kind {
    case yaml.DocumentNode:
        for _, c := range n.Content {
            collectMetadata(c, meta, depth, false)
        }
    case yaml.MappingNode:
        for i := 0; i < len(n.Content); i += 2 {
            key := n.Content[i]
            meta[key.Line] = lineMetadata{depth: depth, inSeq: inSeq}
            if i+1 < len(n.Content) {
                collectMetadata(n.Content[i+1], meta, depth+1, false)
            }
        }
    case yaml.SequenceNode:
        for _, item := range n.Content {
            // Sequence items: the content inside is at the SAME mapping depth
            // but marked as inSeq=true (dash-relative indent).
            collectMetadata(item, meta, depth, true)
        }
    }
}
```

Results for `servers:\n  - host: a\n    port: 80\ndb:\n  host: localhost`:
```
line 1 → {depth: 0, inSeq: false}  (servers)
line 2 → {depth: 1, inSeq: true}   (host — inside seq item)
line 3 → {depth: 1, inSeq: true}   (port — inside seq item)
line 6 → {depth: 0, inSeq: false}  (db)
line 7 → {depth: 1, inSeq: false}  (host — inside mapping, not seq)
```

---

#### New Function: `assignASTMetadata`

Walks tokens, matches each `TokKey` to its line in the metadata map, then propagates `ASTDepth` and `InSeq` to the PRECEDING `TokIndent` token (since reindent operates on indent tokens):

```go
func assignASTMetadata(tokens []Token, meta map[int]lineMetadata) {
    for i := range tokens {
        tokens[i].ASTDepth = -1
        tokens[i].InSeq = false
    }
    if meta == nil {
        return // fallback: leave at -1, reindent will use legacy behavior
    }
    for i := range tokens {
        if tokens[i].Kind != TokKey {
            continue
        }
        lm, ok := meta[tokens[i].Line]
        if !ok {
            continue
        }
        // Set on this key token (for sort).
        tokens[i].ASTDepth = lm.depth
        tokens[i].InSeq = lm.inSeq
        // Also set on the preceding indent token (for reindent).
        indentIdx := findPrecedingIndent(tokens, i)
        if indentIdx >= 0 {
            tokens[indentIdx].ASTDepth = lm.depth
            tokens[indentIdx].InSeq = lm.inSeq
        }
    }
}
```

Note: Lines with `TokDash` but no key (e.g., `- value` as a plain scalar sequence entry) also need metadata. The dash line itself gets depth from the parent sequence:

```go
    // Also handle lines that start with TokDash (sequence items without keys).
    for i := range tokens {
        if tokens[i].Kind != TokDash {
            continue
        }
        // The dash line's indent should be at the parent mapping's depth level.
        // Find its line and check if it's the start of a sequence item.
        indentIdx := findPrecedingIndent(tokens, i)
        if indentIdx >= 0 && tokens[indentIdx].ASTDepth == -1 {
            // Look for the NEXT key on this line or the items below to get context.
            // Actually: dash lines themselves are at the PARENT depth, not child depth.
            // The metadata map has the first key on the dash line.
            // If there's a key after this dash on the same line, use its depth - but
            // the dash INDENT is one level shallower (the dash is at parent indent).
            // Example: "  - name: x" → dash indent is depth 0*width=0+width (the seq is value of depth-0 key)
            // We need: dash indent = (keyDepth - 1) * targetWidth when !inSeq at parent
            //          dash indent = parentDashIndent + 2 when parent is also inSeq
            // This is getting complex. Let me think about this differently.
        }
    }
```

Wait — this is getting complicated because the dash line itself is neither a key nor a value, it's a structural indicator. Let me reconsider.

---

#### Rethinking: What Reindent Actually Needs

Reindent processes EVERY `TokIndent` token and assigns it a new width. The current logic:
```
structural + depth >= 0 → newIndent = depth * targetWidth (or parentIndent+2 for AfterDash)
non-structural → shift by lastDelta
```

With AST metadata, we want:
```
structural + ASTDepth >= 0 + !InSeq → newIndent = computeMapping indent at this depth
structural + ASTDepth >= 0 + InSeq  → newIndent = parent's dash indent + 2
non-structural → shift by lastDelta
```

But what about:
- **Dash lines** (`  - name:`): The indent before the `-` should be at the SEQUENCE position. If the sequence is a value of a depth-0 key, the dash is at `1 * targetWidth`. If the sequence is a value of a depth-1 key, the dash is at `2 * targetWidth`. The metadata for the KEY on this line says `{depth=1, inSeq=true}`, so the indent for the KEY position is `dashIndent + 2`. But the indent token covers the ENTIRE line start (before the dash), not just the key position.

Actually — looking at the token stream again:
```
  3: INDENT(2)   ← this is what reindent modifies
  4: TokDash "- "
  5: TokKey "name"
```

The INDENT is for the entire line. The dash and key are both ON that line. The indent should be set so that the `-` lands at the right column. If this is a depth-1 sequence (value of a depth-0 key), the dash should be at `1 * targetWidth`. The key `name` is then at `dash + 2`.

But the indent token gives the position of the FIRST content on the line (the dash). So:
- Indent token on a dash line → sets the dash position
- Indent token on a key-only line (dash sibling) → sets the key position (= dash position + 2)

The metadata says the KEY is at `{depth=1, inSeq=true}`. For the indent token:
- If the line HAS a dash: indent = depth's sequence indent (how we get from parent to here via a sequence)
- If the line has NO dash but is inSeq: indent = dashIndent + 2

How to compute "depth's sequence indent"? It's: the indent where the parent key lives + targetWidth. If parent key is at indent 0, the sequence starts at indent `targetWidth`.

---

#### Simpler Formulation

Let me define `indentFor(depth, inSeq, hasDashOnLine)`:

```
indentFor(depth=0, inSeq=false, hasDash=false) = 0           (root keys)
indentFor(depth=1, inSeq=false, hasDash=false) = 1*tw        (nested mapping key)
indentFor(depth=1, inSeq=true,  hasDash=true)  = 1*tw        (dash line — dash at parent's child indent)
indentFor(depth=1, inSeq=true,  hasDash=false) = 1*tw + 2    (dash sibling — key continuation)
indentFor(depth=2, inSeq=false, hasDash=false) = 2*tw        (double-nested mapping)
indentFor(depth=2, inSeq=true,  hasDash=true)  = 2*tw        (nested dash line)
indentFor(depth=2, inSeq=true,  hasDash=false) = 2*tw + 2    (nested dash sibling)
```

Pattern: 
- `hasDash && inSeq`: indent = `depth * targetWidth`
- `!hasDash && inSeq`: indent = `depth * targetWidth + 2`
- `!inSeq`: indent = `depth * targetWidth`

Wait, that simplifies to:
- `inSeq && !hasDash`: indent = `depth * targetWidth + 2`
- otherwise: indent = `depth * targetWidth`

Let me verify against prettier output for `items:\n  - name: first\n    value: 1\n` with tw=4:
- `items` (depth=0, !inSeq, !hasDash): `0*4 = 0` ✓
- `- name` line (depth=1, inSeq=true, hasDash=true): `1*4 = 4` → line is `    - name: first` ✓
- `value` line (depth=1, inSeq=true, !hasDash): `1*4 + 2 = 6` → line is `      value: 1` ✓

And for `A:\n- 0:\n` (col-0 value) with tw=4:
- `A` (depth=0, !inSeq, !hasDash): `0*4 = 0` ✓
- `- 0` line (depth=1, inSeq=true, hasDash=true): `1*4 = 4`... but wait, the ORIGINAL is at column 0. After reindent it would move to column 4. Is that correct?

Let me check what prettier does with this:

<br>Actually — `A:\n- 0:` is the PATHOLOGICAL input. After formatting, should it become `A:\n    - 0:\n` (indented under A)? That would be the "correct" formatting. Let me check:

```yaml
A:
- 0:
```
is semantically `{A: [{0: null}]}`. The properly formatted version at tw=4 would be:
```yaml
A:
    - 0:
```

That's what prettier would produce. So yes, `depth=1, inSeq=true, hasDash=true → indent = 1*4 = 4` is correct. The formatter would MOVE the dash from column 0 to column 4, making the structure explicit.

But wait — our current formatter DOESN'T do this for the pathological input because the structuralLines check says line 2 is structural, computeDepths assigns depth based on indent width (0 → depth 0), and it stays at column 0. The AST approach would CORRECTLY reindent it to column 4. That's actually the right behavior — make implicit structure explicit.

Let me verify this won't break the existing test that expects `- 0: 0\n  1:\n` to stay at column 0:

The test `pathological_fuzz` currently expects:
```
input: "- 0: 0\n  1:\n", width=4, want: "- 0: 0\n  1:\n"
```

With AST-derived depths, `- 0: 0` is at depth 0 (it IS a root-level sequence item — no parent mapping). The `1:` is at depth 0, inSeq=true, !hasDash → `0*4 + 2 = 2`. So output would be `- 0: 0\n  1:\n`. Same as current! Good.

But for `A:\n- 0:\n`, the `-` is at depth 1 (inside A's value). So it would get indented to 4. The original test that triggered the bug would now produce DIFFERENT (but correct) output: `A:\n    - 0:\n`.

This IS a behavior change for the column-0 value block case. But it's the CORRECT behavior — making the structure explicit.

---

#### Final Formula

For each structural `TokIndent` token with AST metadata:

```go
func computeNewIndent(astDepth int, inSeq bool, hasDash bool, targetWidth int) int {
    base := astDepth * targetWidth
    if inSeq && !hasDash {
        return base + 2
    }
    return base
}
```

That's it. One formula, driven entirely by the AST. No heuristics, no AfterDash special case, no indent-width stack.

---

#### Detecting `hasDash` on a Line

Look forward from the indent token on the same line:

```go
func lineHasDash(tokens []Token, indentIdx int) bool {
    for j := indentIdx + 1; j < len(tokens); j++ {
        if tokens[j].Kind == TokNewline { return false }
        if tokens[j].Kind == TokDash { return true }
    }
    return false
}
```

(This function already exists from the AfterDash implementation.)

---

#### What Gets Deleted

- `computeDepths` function (the indent-width stack heuristic) — DELETED
- `recomputeDepths` function — DELETED
- `AfterDash` field on Token — DELETED
- `Depth` field on Token — REPLACED by `ASTDepth`
- `newIndentAtDepth` map in reindentTokens — DELETED (no longer needed)
- Root dash detection in computeDepths — DELETED
- The `recomputeDepths(tokens)` call after sortKeys in printFormatted — REPLACED

---

#### What Gets Added

- `buildASTMetadata(src []byte) map[int]lineMetadata` — walks Node tree
- `collectMetadata(n *yaml.Node, meta map[int]lineMetadata, depth int, inSeq bool)` — recursive walker
- `assignASTMetadata(tokens []Token, meta map[int]lineMetadata)` — sets ASTDepth/InSeq on tokens
- `lineMetadata` struct — `{depth int, inSeq bool}`

---

#### Modified Functions

**`annotate(tokens []Token, src []byte)`**:
```go
func annotate(tokens []Token, src []byte) {
    structuralLines := buildStructuralLineSet(src)
    astMeta := buildASTMetadata(src)

    line := 1
    for i := range tokens {
        tokens[i].ASTDepth = -1
        tokens[i].Line = line

        if tokens[i].Kind == TokIndent {
            if i+1 < len(tokens) && tokens[i+1].Kind == TokNewline {
                for _, b := range tokens[i].Raw { if b == '\n' { line++ } }
                continue
            }
            if i+1 < len(tokens) && tokens[i+1].Kind == TokComment {
                tokens[i].Structural = true
            } else {
                tokens[i].Structural = structuralLines == nil || structuralLines[line]
            }
        }

        for _, b := range tokens[i].Raw { if b == '\n' { line++ } }
    }

    assignASTMetadata(tokens, astMeta)
}
```

No more `computeDepths` call.

**`reindentTokens(tokens []Token, targetWidth int)`**:
```go
func reindentTokens(tokens []Token, targetWidth int) {
    lastDelta := 0

    for i := range tokens {
        if tokens[i].Kind != TokIndent { continue }
        oldIndent := len(tokens[i].Raw)

        var newIndent int
        if tokens[i].Structural && tokens[i].ASTDepth >= 0 {
            hasDash := lineHasDash(tokens, i)
            newIndent = computeNewIndent(tokens[i].ASTDepth, tokens[i].InSeq, hasDash, targetWidth)
            lastDelta = newIndent - oldIndent
        } else {
            newIndent = oldIndent + lastDelta
            if newIndent < 0 { newIndent = 0 }
        }

        tokens[i].Raw = []byte(strings.Repeat(" ", newIndent))
        delta := newIndent - oldIndent

        // Shift block scalar content by same delta.
        if delta != 0 {
            for j := i + 1; j < len(tokens); j++ {
                if tokens[j].Kind == TokNewline { break }
                if tokens[j].Kind == TokBlockScalar {
                    tokens[j].Raw = shiftBlockScalarIndent(tokens[j].Raw, delta)
                    break
                }
            }
        }
    }
}

func computeNewIndent(astDepth int, inSeq, hasDash bool, targetWidth int) int {
    base := astDepth * targetWidth
    if inSeq && !hasDash {
        return base + 2
    }
    return base
}
```

**`groupEntries(tokens []Token, from, to, targetDepth int)`**:
```go
func groupEntries(tokens []Token, from, to, targetDepth int) []mappingEntry {
    var entries []mappingEntry

    for i := from; i < to; i++ {
        if tokens[i].Kind != TokKey {
            continue
        }
        // Use AST-derived depth for grouping.
        if tokens[i].ASTDepth != targetDepth {
            continue
        }

        entries = append(entries, mappingEntry{
            startIdx: findEntryStart(tokens, i),
            key:      string(tokens[i].Raw),
        })
    }

    // Set endIdx (unchanged logic).
    for i := range entries {
        if i+1 < len(entries) {
            entries[i].endIdx = entries[i+1].startIdx
        } else {
            end := to
            for end > entries[i].startIdx {
                tok := tokens[end-1]
                if tok.Kind != TokIndent && tok.Kind != TokNewline && tok.Kind != TokSpace {
                    break
                }
                end--
            }
            if end < to && tokens[end].Kind == TokNewline {
                end++
            }
            entries[i].endIdx = end
        }
    }
    return entries
}
```

No more `findPrecedingIndent` + Depth lookup. Just `tokens[i].ASTDepth`.

**`sortKeysAtDepth`**:
```go
func sortKeysAtDepth(tokens []Token, targetDepth, from, to int) []Token {
    entries := groupEntries(tokens, from, to, targetDepth)

    if len(entries) >= 2 && !hasAnchorAliasDependency(tokens, entries) {
        tokens = reorderEntries(tokens, entries)
        // ASTDepth is position-invariant (semantic, not physical).
        // No recomputation needed.
        entries = groupEntries(tokens, from, to, targetDepth)
    }

    for _, e := range entries {
        tokens = sortKeysAtDepth(tokens, targetDepth+1, e.startIdx, e.endIdx)
    }
    return tokens
}
```

**`printFormatted`** — remove `recomputeDepths(tokens)` after sortKeys:
```go
if opts.SortKeys {
    tokens = sortKeys(tokens)
    // No recomputeDepths needed — ASTDepth is invariant under reordering.
}
```

---

#### Handling Comments (for reindent)

Comments don't appear in the AST. Their indent tokens get `ASTDepth = -1`. Current behavior: comments are marked `Structural = true` and shift with the delta of the nearest structural line. With the new approach, they continue to work via `lastDelta` (the non-structural/fallback path).

This is correct: a comment's indent follows its surrounding context, which is captured by `lastDelta` from the previous structural line.

---

#### Fallback (AST parse failure)

If `buildASTMetadata` returns nil (YAML parse failed — shouldn't happen since we validate first, but defensive), `assignASTMetadata` leaves all `ASTDepth = -1`. The reindent step treats ALL tokens as non-structural (shift by delta = 0), which means the file passes through unchanged. This is safe — if we can't understand the structure, don't modify it.

---

#### Behavior Changes From Current

1. **Column-0 value blocks get properly indented**: `A:\n- 0:` becomes `A:\n    - 0:\n` (with tw=4). This is correct — makes implicit structure explicit.

2. **Sort correctly identifies siblings**: `workers` and `database` are sorted together; `queue` inside workers' sequence is NOT grouped with root keys.

3. **Reindent no longer depends on input indentation heuristics**: Two files with the same AST structure but different original indentation produce identical output. (This was already mostly true, but edge cases around column-0 values are now handled.)

---

#### Test Strategy

1. All 107 existing stress tests must pass (most files are normally indented — AST depth matches what indent-depth computed)
2. Real-world corpus (11 repo files) must pass
3. Pathological inputs:
   - `- 0: 0\n  1:` with tw=4 → idempotent (root-level seq, stays at col 0)
   - `A:\n- 0:` with SortKeys → correctly NOT sorted (0 is at depth 1, not depth 0)
   - `workers:\n- queue...\ndatabase:...` with SortKeys → database sorts before workers
4. Prettier comparison: `items:\n  - name:\n    value:` with tw=4 → matches prettier
5. Fuzz 45s with options

---

#### Touch Surface

| File | Lines changed (est) | Risk |
|------|-------------------|------|
| `tokenizer.go` | ~5 (struct fields) | Zero — additive |
| `printer.go` | ~120 (delete computeDepths/recomputeDepths/AfterDash logic: -60, add new functions: +80, modify annotate/reindent/groupEntries/sortKeysAtDepth: ~40) | Medium — reindent behavior changes, sort behavior changes |
| `yaml_test.go` | ~20 (update TestDashReindent expected outputs if col-0 value blocks now get indented) | Low |
| `stress_format_test.go` | ~5 (update/add test cases) | Low |

Total: ~150 lines net change in printer.go. The file is currently ~760 lines.

---

### Tasks for 7C.3

- [ ] 7C.3.1: Add `ASTDepth int`, `InSeq bool`, `Line int` to Token struct. Remove `Depth int`, `AfterDash bool`.
- [ ] 7C.3.2: Add `lineMetadata` struct, `buildASTMetadata`, `collectMetadata` functions
- [ ] 7C.3.3: Add `assignASTMetadata` function
- [ ] 7C.3.4: Rewrite `annotate()` — remove computeDepths call, add buildASTMetadata + assignASTMetadata
- [ ] 7C.3.5: Add `computeNewIndent(astDepth, inSeq, hasDash, targetWidth) int` function
- [ ] 7C.3.6: Rewrite `reindentTokens` — use ASTDepth/InSeq/hasDash instead of Depth/AfterDash/newIndentAtDepth map
- [ ] 7C.3.7: Rewrite `groupEntries` — use `tokens[i].ASTDepth` instead of indent Depth lookup
- [ ] 7C.3.8: Simplify `sortKeysAtDepth` — remove `computeDepths(tokens)` call after reorder
- [ ] 7C.3.9: Simplify `printFormatted` — remove `recomputeDepths(tokens)` after sortKeys
- [ ] 7C.3.10: Delete `computeDepths`, `recomputeDepths` functions
- [ ] 7C.3.11: Delete `lineHasDash` from computeDepths (keep it as standalone helper for reindent)
- [ ] 7C.3.12: Update all references to `.Depth` → `.ASTDepth` in printer.go
- [ ] 7C.3.13: Fix compilation errors (any other files referencing Depth/AfterDash)
- [ ] 7C.3.14: Update TestDashReindent expectations (col-0 value blocks may now indent correctly)
- [ ] 7C.3.15: Add test: `A:\n- 0:` with SortKeys — key `0` NOT grouped at root depth
- [ ] 7C.3.16: Add test: `workers:\n- queue: high\n  concurrency: 5\ndatabase:\n  host: localhost` with SortKeys
- [ ] 7C.3.17: Run all YAML tests — zero regressions on normally-indented files
- [ ] 7C.3.18: Run stress tests + real-world corpus
- [ ] 7C.3.19: Add fuzz corpus entries back, verify they pass
- [ ] 7C.3.20: Fuzz 45s with options
- [ ] 7C.3.21: Pipeline verification (vet, fmt, lint)

## Phase 10: YAML Inline Spacing Normalization

**Date**: 2026-07-15
**Priority**: HIGH — without this, users still need prettier/yamlfmt for basic consistency
**Competition**: prettier and yamlfmt both normalize inline spacing

---

### Problem

After formatting with cfv, YAML files can still have inconsistent spacing:
```yaml
name:    my-app
version:  1.0.0
flow: {a:  1, b:   2}
```
Every other formatter normalizes this. Users perceive a formatter that leaves inconsistent spacing as broken.

---

### Two Operations

#### Operation 1: Strip leading whitespace from block values

`key:    value` → `key: value`

The excess spaces live as leading whitespace in `TokValue.Raw`. The colon token already emits `": "` (one space). The value token has `"   value"` (extra spaces + content).

#### Operation 2: Re-serialize flow collections from the AST

`{a:  1, b:   2}` → `{a: 1, b: 2}`

Flow collections are currently opaque `TokFlow` tokens. Instead of byte-walking them with a hack, use the yaml.v3 Node tree (which fully parses flow content with correct semantics) to re-serialize them with normalized spacing.

---

### Why AST-driven flow re-serialization (not byte walking)

1. **YAML semantics are subtle in flow context.** `b:2` (no space after colon) is a PLAIN SCALAR, not key `b` with value `2`. A byte walker would have to re-implement YAML's flow parsing rules to know the difference. The parser already did this work.

2. **Same principle as the depth refactor.** The AST IS the structure. Don't guess from bytes when the parser tells you the answer.

3. **Extensible.** Once we have AST-driven flow serialization, we can add print-width-based line breaking (break long flows into multiline) in a future phase. A byte walker can't do that.

---

### Architecture

```
printFormatted:
    buildASTMetadata(src)           ← already exists
    annotate(tokens, src, astMeta)  ← already exists
    normalizeValueSpacing(tokens)   ← NEW (Operation 1 — trivial)
    normalizeFlowTokens(tokens, src) ← NEW (Operation 2 — AST-driven)
    sortKeys (if requested)
    reindentTokens
    ... rest unchanged
```

---

### Operation 1: `normalizeValueSpacing`

```go
// normalizeValueSpacing strips leading horizontal whitespace from TokValue
// tokens that follow a TokColon. This normalizes "key:    value" to "key: value".
// The whitespace between : and the value is insignificant in YAML.
// Internal whitespace within the value is preserved.
func normalizeValueSpacing(tokens []Token) {
    for i := range tokens {
        if tokens[i].Kind != TokValue {
            continue
        }
        if i == 0 || tokens[i-1].Kind != TokColon {
            continue // Only strip values after colons, not continuation lines
        }
        tokens[i].Raw = bytes.TrimLeft(tokens[i].Raw, " \t")
    }
}
```

**Edge cases**:
- `key:` (no value) — no TokValue emitted, no-op ✓
- `key: value` (already correct) — TrimLeft on `"value"` is no-op ✓
- `key:  "quoted"` — value raw is `" \"quoted\""` → becomes `"\"quoted\""` ✓
- `key:   &anchor val` — value raw is `"  &anchor val"` → becomes `"&anchor val"` ✓
- `key:   !!str 42` — value raw is `"  !!str 42"` → becomes `"!!str 42"` ✓
- Continuation line (multi-line plain scalar) — NOT preceded by TokColon, skipped ✓

**Semantic safety**: Verified — `yaml.Unmarshal("key:    value")` == `yaml.Unmarshal("key: value")`.

---

### Operation 2: `normalizeFlowTokens` — AST-driven flow re-serialization

**Approach**: For each `TokFlow` token in the stream, find the corresponding Node in the yaml.v3 tree (by matching line:column), then re-serialize that Node into normalized flow syntax, replacing the `TokFlow.Raw`.

**Why line:column matching works**: The tokenizer records where each TokFlow starts. The yaml.v3 Node tree records Line and Column for each node. A flow map `{...}` at line 3 column 10 in the tokens corresponds to the Node at Line=3 Column=10 with Style=FlowStyle.

#### Building the flow node map

Extend `buildASTMetadata` (or add a companion function) to build a map from `(line, column) → *yaml.Node` for all flow-style nodes:

```go
type flowNodeMap map[[2]int]*yaml.Node // [line, column] → node

func buildFlowNodeMap(src []byte) flowNodeMap {
    if len(src) > 0 && src[len(src)-1] != '\n' {
        src = append(bytes.Clone(src), '\n')
    }
    var root yaml.Node
    if err := yaml.Unmarshal(src, &root); err != nil {
        return nil
    }
    m := make(flowNodeMap)
    collectFlowNodes(&root, m)
    return m
}

func collectFlowNodes(n *yaml.Node, m flowNodeMap) {
    if (n.Kind == yaml.MappingNode || n.Kind == yaml.SequenceNode) && n.Style == yaml.FlowStyle {
        m[[2]int{n.Line, n.Column}] = n
    }
    for _, c := range n.Content {
        collectFlowNodes(c, m)
    }
}
```

#### Re-serializing a flow Node

```go
// serializeFlowNode produces normalized flow syntax from a yaml.Node.
// Output style: {key: value, key2: value2} (compact braces, single space after : and ,)
func serializeFlowNode(n *yaml.Node) []byte {
    var buf bytes.Buffer
    writeFlowNode(&buf, n)
    return buf.Bytes()
}

func writeFlowNode(buf *bytes.Buffer, n *yaml.Node) {
    switch n.Kind {
    case yaml.MappingNode:
        buf.WriteByte('{')
        for i := 0; i < len(n.Content); i += 2 {
            if i > 0 {
                buf.WriteString(", ")
            }
            writeFlowNode(buf, n.Content[i])   // key
            buf.WriteString(": ")
            writeFlowNode(buf, n.Content[i+1]) // value
        }
        buf.WriteByte('}')

    case yaml.SequenceNode:
        buf.WriteByte('[')
        for i, item := range n.Content {
            if i > 0 {
                buf.WriteString(", ")
            }
            writeFlowNode(buf, item)
        }
        buf.WriteByte(']')

    case yaml.ScalarNode:
        writeFlowScalar(buf, n)

    case yaml.AliasNode:
        buf.WriteByte('*')
        buf.WriteString(n.Value)
    }
}

func writeFlowScalar(buf *bytes.Buffer, n *yaml.Node) {
    switch n.Style {
    case yaml.DoubleQuotedStyle:
        // Re-emit with double quotes. Use the Node.Value (unescaped) and
        // re-escape for YAML double-quote rules.
        buf.WriteByte('"')
        buf.WriteString(escapeDoubleQuoted(n.Value))
        buf.WriteByte('"')
    case yaml.SingleQuotedStyle:
        buf.WriteByte('\'')
        buf.WriteString(escapeSingleQuoted(n.Value))
        buf.WriteByte('\'')
    default:
        // Plain scalar. For flow context, some values need quoting
        // (e.g., values containing `:`, `,`, `{`, `}`, `[`, `]`).
        // The Node.Tag tells us the type; Value is the unescaped content.
        // If the value is safe as a plain scalar in flow context, emit plain.
        // Otherwise, double-quote it.
        if needsQuotingInFlow(n.Value, n.Tag) {
            buf.WriteByte('"')
            buf.WriteString(escapeDoubleQuoted(n.Value))
            buf.WriteByte('"')
        } else {
            buf.WriteString(n.Value)
        }
    }
}
```

#### Helper functions

```go
// escapeDoubleQuoted escapes a string for YAML double-quoted style.
func escapeDoubleQuoted(s string) string {
    // Replace: \ → \\, " → \", newline → \n, tab → \t, etc.
    var b strings.Builder
    for _, r := range s {
        switch r {
        case '\\': b.WriteString(`\\`)
        case '"':  b.WriteString(`\"`)
        case '\n': b.WriteString(`\n`)
        case '\t': b.WriteString(`\t`)
        case '\r': b.WriteString(`\r`)
        default:   b.WriteRune(r)
        }
    }
    return b.String()
}

// escapeSingleQuoted escapes a string for YAML single-quoted style.
// Only escape needed: ' → ''
func escapeSingleQuoted(s string) string {
    return strings.ReplaceAll(s, "'", "''")
}

// needsQuotingInFlow returns true if a plain scalar value contains characters
// that are ambiguous in flow context and must be quoted.
func needsQuotingInFlow(value, tag string) bool {
    if value == "" {
        return true // empty string needs quotes in flow
    }
    // Characters that are flow indicators or could be misinterpreted.
    for _, r := range value {
        switch r {
        case '{', '}', '[', ']', ',', ':', '#', '&', '*', '!', '|', '>', '\'', '"', '%', '@', '`':
            return true
        }
    }
    // Values that look like other types need quoting to preserve string type.
    if tag == "!!str" {
        switch value {
        case "true", "false", "null", "~", "yes", "no", "on", "off":
            return true
        }
        // Check if it looks like a number.
        if looksNumeric(value) {
            return true
        }
    }
    return false
}

// looksNumeric checks if a string would be parsed as a number by YAML.
func looksNumeric(s string) bool {
    if len(s) == 0 { return false }
    // Simple check: starts with digit, -, or .
    if s[0] == '-' || s[0] == '+' || s[0] == '.' || (s[0] >= '0' && s[0] <= '9') {
        // Try to parse as int or float.
        // Use a simple heuristic: if all chars are digits, dots, e, E, -, +
        for _, r := range s {
            if !((r >= '0' && r <= '9') || r == '.' || r == 'e' || r == 'E' || r == '-' || r == '+' || r == '_') {
                return false
            }
        }
        return true
    }
    return false
}
```

#### Connecting to the token stream

```go
// normalizeFlowTokens replaces each TokFlow token's Raw with AST-driven
// re-serialized content. Uses the yaml.v3 Node tree for correct semantics.
func normalizeFlowTokens(tokens []Token, flowNodes flowNodeMap) {
    if flowNodes == nil {
        return // parse failed — leave flows unchanged
    }
    for i := range tokens {
        if tokens[i].Kind != TokFlow {
            continue
        }
        node, ok := flowNodes[[2]int{tokens[i].Line, findFlowColumn(tokens, i)}]
        if !ok {
            continue // no matching node — leave unchanged
        }
        tokens[i].Raw = serializeFlowNode(node)
    }
}

// findFlowColumn computes the column (1-based) of a TokFlow token by
// summing the widths of tokens on the same line before it.
func findFlowColumn(tokens []Token, flowIdx int) int {
    col := 1
    for j := flowIdx - 1; j >= 0; j-- {
        if tokens[j].Kind == TokNewline {
            break
        }
        col += len(tokens[j].Raw)
    }
    return col
}
```

Wait — there's a subtlety. The column computation from tokens may not match the Node tree's column exactly, because the Node tree uses the ORIGINAL source positions, and by the time we call `normalizeFlowTokens`, other normalization steps may have shifted columns.

**Fix**: Call `normalizeFlowTokens` BEFORE any other modifications to the token stream (before `normalizeValueSpacing` and before sort/reindent). At that point, token positions still match the original source.

Actually better: build the flow node map during `annotate` (which runs on the original source) and set `TokFlow.Line` during the line-counting pass. Then matching is straightforward: `flowNodes[[2]int{tokens[i].Line, originalColumn}]`.

But computing `originalColumn` requires knowing the position in the original source, which we track during tokenization. Let me check if the tokenizer records position:

Actually — the simplest correct approach: record the **start byte offset** of each TokFlow during tokenization, and also record the byte offset of each flow Node from the source. But yaml.Node only gives Line:Column, not byte offset.

Let me reconsider. The matching problem:
- TokFlow at line L, column C (computable from tokens)
- yaml.Node at Line L, Column C (from Node tree)
- These WILL match because both refer to the same source bytes and we haven't modified anything yet

The column for the TokFlow token can be precomputed during `annotate`'s line-counting pass. Add a `Column int` field to Token? Or compute it on-the-fly in `normalizeFlowTokens`.

The simplest: compute column from the token positions (sum lengths on current line). Since we run this FIRST (before any modifications), the positions match the source. The `findFlowColumn` function above is correct for this.

**Revised pipeline order**:
```
printFormatted:
    buildASTMetadata(src)
    buildFlowNodeMap(src)            ← NEW
    annotate(tokens, src, astMeta)
    normalizeFlowTokens(tokens, flowNodes)  ← NEW (first — before any token modification)
    normalizeValueSpacing(tokens)    ← NEW (after flows, so flow values don't get double-processed)
    sortKeys (if requested)
    reindentTokens
    ... rest unchanged
```

---

### Anchor handling in flow collections

If a flow collection contains an anchor: `{a: &ref 1, b: *ref}`, the Node tree has:
- Key `a` → Scalar with Anchor="ref", Value="1"
- Key `b` → AliasNode pointing to the anchor

`writeFlowNode` handles AliasNode by emitting `*name`. For anchors, check `n.Anchor` field:

```go
case yaml.ScalarNode:
    if n.Anchor != "" {
        buf.WriteByte('&')
        buf.WriteString(n.Anchor)
        buf.WriteByte(' ')
    }
    writeFlowScalar(buf, n)
```

---

### Empty flow collections

`{}` and `[]` — the Node has zero Content items. `writeFlowNode` emits `{}` or `[]` (no space inside). Matches both prettier and yamlfmt.

---

### Null values in flow

`{key: null}` or `{key: }` — Node has a ScalarNode with Tag=`!!null` and Value="" or "null". 

For null values in flow, prettier emits `null` explicitly. yamlfmt does too. Our serializer should emit `null` for null-tagged scalars:

```go
func writeFlowScalar(buf *bytes.Buffer, n *yaml.Node) {
    if n.Tag == "!!null" {
        buf.WriteString("null")
        return
    }
    // ... rest of style-based logic
}
```

---

### Comments inside flow collections

yaml.v3 preserves comments on Nodes (`HeadComment`, `LineComment`, `FootComment`). However, formatting flow collections inline doesn't have a good place for comments. prettier DROPS comments inside flow collections (rewrites them without the comment). yamlfmt preserves them.

**Our approach**: If any node in the flow tree has a comment, **skip re-serialization** for that TokFlow token (leave it unchanged). This is conservative — we don't corrupt data, we just don't normalize spacing for flows with comments. This is rare (comments in flow collections are unusual).

```go
func hasComments(n *yaml.Node) bool {
    if n.HeadComment != "" || n.LineComment != "" || n.FootComment != "" {
        return true
    }
    for _, c := range n.Content {
        if hasComments(c) {
            return true
        }
    }
    return false
}
```

In `normalizeFlowTokens`:
```go
if hasComments(node) {
    continue // skip — preserve original formatting for flows with comments
}
```

---

### Multi-line flow collections

Some flow collections span multiple lines:
```yaml
config: {
  key1: value1,
  key2: value2
}
```

After re-serialization, this becomes a single line: `config: {key1: value1, key2: value2}`. This is correct — the formatter normalizes to compact flow style. If the user wants multiline, they should use block style.

But wait — the TokFlow token captures the multi-line content including newlines. The replacement (`serializeFlowNode`) produces a single line. This means subsequent tokens' line numbers will be wrong. Does this matter?

It matters for `normalizeFlowTokens` matching OTHER flow tokens on later lines. But since we're iterating forward and each TokFlow is matched independently by its OWN line/column (which hasn't changed yet at time of access), this is fine. After replacement, line numbers are stale but we don't use them again (annotate already ran, ASTDepth already set).

For reindent: `TokFlow` tokens don't have `TokIndent` before them (they appear inline after a colon). They don't participate in reindentation. So stale line numbers don't affect reindent.

---

### Semantic equivalence guarantee

The re-serialized flow produces the same parsed value as the original because:
1. We use the PARSED Node tree (which IS the semantic value)
2. We serialize FROM that tree (no information loss)
3. Quoting rules ensure round-trip safety (needsQuotingInFlow)

This is proven by our existing stress test framework: `yaml.Unmarshal(original) == yaml.Unmarshal(formatted)`.

---

### Test strategy

1. **Unit test**: `TestNormalizeValueSpacing` — verify stripping on all edge cases
2. **Unit test**: `TestNormalizeFlowCollections` — verify re-serialization matches expected:
   - `{a:  1, b:   2}` → `{a: 1, b: 2}`
   - `{key: "value with : colon"}` → `{key: "value with : colon"}`
   - `{a: {b: 1}, c: [1, 2]}` → `{a: {b: 1}, c: [1, 2]}`
   - `{}` → `{}`
   - `[1,  2,   3]` → `[1, 2, 3]`
   - `{a: &ref 1, b: *ref}` → `{a: &ref 1, b: *ref}`
3. **Stress test cases**: Add inputs with messy spacing, verify semantic equivalence
4. **Prettier comparison**: Format same input, verify our output matches prettier's spacing logic
5. **All existing tests**: Must produce identical output (they already have correct spacing)
6. **Real-world corpus**: Must pass
7. **Fuzz 45s**: Verify idempotency with normalization active

---

### Files

- `pkg/formatter/yamlfmt/printer.go`:
  - Add `normalizeValueSpacing`
  - Add `normalizeFlowTokens`, `buildFlowNodeMap`, `collectFlowNodes`
  - Add `serializeFlowNode`, `writeFlowNode`, `writeFlowScalar`
  - Add `escapeDoubleQuoted`, `escapeSingleQuoted`, `needsQuotingInFlow`, `looksNumeric`
  - Add `hasComments`, `findFlowColumn`
  - Modify `printFormatted` pipeline to call both normalizations
- `pkg/formatter/yamlfmt/yaml_test.go`: Unit tests
- `cmd/cfv/stress_format_test.go`: Stress test cases

---

### Tasks

- [ ] 10.1: Implement `normalizeValueSpacing` (trim leading whitespace from TokValue after TokColon)
- [ ] 10.2: Implement `buildFlowNodeMap` and `collectFlowNodes`
- [ ] 10.3: Implement `serializeFlowNode`, `writeFlowNode`, `writeFlowScalar`
- [ ] 10.4: Implement `escapeDoubleQuoted`, `escapeSingleQuoted`, `needsQuotingInFlow`, `looksNumeric`
- [ ] 10.5: Implement `hasComments` (skip re-serialization for flows with comments)
- [ ] 10.6: Implement `normalizeFlowTokens` with `findFlowColumn` for line:column matching
- [ ] 10.7: Wire into `printFormatted`: build flow map, call normalizeFlowTokens then normalizeValueSpacing
- [ ] 10.8: Add unit tests for value spacing normalization (10+ edge cases)
- [ ] 10.9: Add unit tests for flow re-serialization (10+ cases including nested, anchors, quoting)
- [ ] 10.10: Add stress test cases with semantic equivalence verification
- [ ] 10.11: Compare output against prettier for test inputs
- [ ] 10.12: Run all YAML tests — zero regressions
- [ ] 10.13: Run stress tests + real-world corpus
- [ ] 10.14: Fuzz 45s with options — zero failures
- [ ] 10.15: Pipeline verification (vet, fmt, lint, build, test)

## Phase 11: Fix Tokenizer Bugs + Add Semantic Fuzz Assertions

**Date**: 2026-07-15
**Context**: Fuzzing with option combinations found 3 bugs in TOML, INI, Properties tokenizers/printers. These are specific implementation mistakes in boundary handling, not architectural flaws. The fix is to correct each bug AND add semantic equivalence checking to all fuzz targets so future bugs are caught immediately.

---

### 11.1: TOML — Comment attached to wrong entry after multiline string

**Input**: `0="""\n"""\n#` (key `0` with multiline string, followed by comment)
**With SortKeys**: Output is `#0 = """\n"""` — comment `#` prepended to the key name
**Root cause**: The TOML grouper (printer.go `sortGroups`) attaches comments to the NEXT entry. The comment `#` on line 3 has no following entry, so it becomes a "trailing comment." But somehow during sort, it gets prepended to the key entry.

**Detailed trace**:
The grouper produces groups from the token stream:
- Group 1: `GroupEntry` for `0="""\n"""\n` (key-value with multiline string)
- Group 2: `GroupComment` for `#`

In `sortGroups`, when a `GroupComment` is encountered, it's stored in `commentRun` waiting to attach to the next entry. If NO entry follows (it's a trailing comment), the flush logic at the end emits it. But with SortKeys reordering, the comment somehow ends up in the wrong position.

Looking at the `sortGroups` logic:
```go
case GroupComment:
    commentRun = append(commentRun, group)
```
Then at the end:
```go
if len(commentRun) > 0 {
    result = append(result, commentRun...)
}
flushEntries()
```

The trailing comments are emitted BEFORE `flushEntries()`. If there's one entry in `entryRun`, the comment goes first, then the entry. Output: comment, then entry = `#\n0 = """\n"""`. But the output shows `#0 = ...` — no newline between comment and key. The issue might be that the comment token doesn't include a trailing newline, so when printed it butts up against the key.

**Fix**: In `sortGroups`, trailing comments (commentRun with no following entry) should be emitted AFTER the entries, not before:

```go
// At end of sortGroups:
flushEntries()  // entries first
if len(commentRun) > 0 {
    result = append(result, commentRun...)  // trailing comments after
}
```

Current code has these reversed. Swap them.

**Also**: Verify that the comment token includes a newline in its raw, or that the printer adds one. If the comment raw is just `#` without a newline (because it's at EOF without trailing newline), the printer must handle this.

**Test**: Re-add fuzz corpus entry, verify passes. Add explicit test with multiline string + trailing comment.

**Files**: `pkg/formatter/tomlfmt/printer.go` (sortGroups trailing comment order)

---

### 11.2: INI — SortKeys non-idempotent with escaped backslash keys

**Input**: `A:\n\=00\n\=\` (colon separator, keys are `A`, `\`, `\`)
**With SortKeys**: First sort produces `\=00\n\=\\nA : `, second produces `\=\\\nA : \n\=00`
**Root cause**: Two keys have the SAME name (`\`) but different values (`00` and `\`). With `SortKeys`, equal keys should maintain their original order (stable sort). But the sort is not stable for these entries, OR the key extraction is including the value in the comparison.

**Detailed analysis**:
The INI has three entries in the default section:
- `A` : `` (empty value, colon separator)
- `\` = `00`
- `\` = `\`

After sort: `\`, `\`, `A` (sorted alphabetically). The two `\` entries should maintain original order (`\=00` before `\=\`). But on second pass they flip.

The issue: `sortEntries` in `pkg/formatter/inifmt/printer.go` uses `slices.SortStableFunc`. If the comparison function extracts the key correctly (just `\` for both), they should be stable. Let me check what `sortEntries` compares:

```go
func sortEntries(entries []Entry) []Entry {
    sorted := make([]Entry, len(entries))
    copy(sorted, entries)
    slices.SortStableFunc(sorted, func(a, b Entry) int {
        return strings.Compare(a.Key.Raw, b.Key.Raw)  // ← comparing Raw?
    })
    return sorted
}
```

If it compares `Key.Raw` and both are `\` (one byte: backslash), they should be equal and stable. But wait — the `Key.Raw` for an INI key with backslash might include the escape prefix differently on first vs. second pass.

Actually the real issue might be simpler: after formatting, the separator gets normalized (`:` → ` = `), changing the byte content. On the second pass, the tokenizer re-parses and might extract different key boundaries.

Let me check: on first pass, input is `A:\n\=00\n\=\`. The tokenizer parses:
- Key `A`, Sep `:`, Value `` 
- Key `\`, Sep `=`, Value `00`
- Key `\`, Sep `=`, Value `\`

After sort + format: `\=00\n\=\\nA : `. On second pass:
- Key `\`, Sep `=`, Value `00`
- Key `\`, Sep `=`, Value `\` followed by `\nA : ` — wait, does `\` at end of value look like a continuation? 

INI doesn't have continuation lines. But the value `\` at end of file (with FinalNewline=false) might be the issue — the tokenizer might be consuming `\nA` as part of the previous value because it misidentifies the `\` as an escape for the newline.

**Fix**: The INI tokenizer shouldn't treat `\` + newline as an escape sequence for VALUES. INI values are literal (the spec doesn't define escape sequences in values — that's a Properties thing). Check if our INI tokenizer is incorrectly applying escape logic from the Properties world.

**Check**: Read `pkg/formatter/inifmt/lexer.go` to see if it handles `\` specially in values.

**Test**: Re-add corpus entry, verify fix is idempotent. Add explicit test with backslash keys.

**Files**: `pkg/formatter/inifmt/lexer.go` (value parsing, potential incorrect escape handling)

---

### 11.3: Properties — Continuation to empty resolves to backslash literal

**Input**: `0 \\\n` = key `0`, space separator, value `\` + newline (continuation marker)
**magiconair parses**: key=`0`, value=`""` (empty — continuation to nothing)
**Our formatter outputs**: `0 = \` (value is a literal backslash — WRONG)
**Root cause**: The tokenizer captures `\\\n` (bytes: `\`, `\n`) as the value raw. The printer emits this verbatim. When FinalNewline=false strips the trailing `\n`, we're left with `0 = \` — which parses as a continuation, not as an empty value.

The real issue: the tokenizer correctly identifies this as a continuation (that's how it avoids looping — the EOF fix). But the VALUE token's raw bytes include the continuation marker (`\` + `\n`) even though the semantic value is empty. The printer then emits these bytes, producing output with different semantics.

**Fix**: The printer should DECODE continuation values and re-encode them properly. When a value token's raw consists entirely of continuation markers with no actual content, the decoded value is empty, and the output should be empty (no raw bytes for the value):

In the grouper/printer where entries are built:
```go
// If the value raw is purely a continuation to nothing (decoded value is empty),
// emit nothing for the value.
if isEmptyContinuation(e.value.Raw) {
    // Don't emit value — it's semantically empty
} else {
    buf.Write(e.value.Raw)
}
```

`isEmptyContinuation`: Check if the value raw bytes, when decoded (resolving continuations), produce an empty string. A continuation is `\` + newline + optional leading whitespace on the next line. If after resolving all continuations the result is empty, the value is empty.

Actually simpler: the issue is specifically `\` + EOF (or `\` + newline at the end with nothing after). The fix from earlier (breaking at EOF) prevents the infinite loop but still captures `\\\n` as value raw. The fix should be: **if the continuation leads to EOF with no content, trim the continuation marker from the value raw.**

In `tokenizeKeyValue`, after the continuation loop, check if the value is ONLY continuation markers with no content:
```go
// After the value loop:
if *pos > valueStart {
    valueRaw := src[valueStart:*pos]
    // If the value is entirely a continuation to nothing (last chars are \+newline
    // with no non-whitespace content after), trim to actual content.
    decoded := decodeContinuations(valueRaw)
    if len(decoded) == 0 {
        // Value is semantically empty — don't emit a value token.
    } else {
        tokens = append(tokens, Token{Kind: TokValue, Raw: valueRaw})
    }
}
```

Wait, that changes the token stream structure. The printer expects a value token to be present (or not). If we suppress it, that's clean — an empty value has no TokValue token.

But actually, this ONLY matters when FinalNewline=false strips the trailing newline from a continuation. With FinalNewline=true (default), the output is `0 = \\\n` which is a valid continuation to an empty next line — and re-parses correctly.

**Simpler fix**: In the printer, when emitting a value that ends with a continuation marker (`\` + newline), and FinalNewline=false would strip that newline, don't emit the continuation — emit the decoded value instead.

Even simpler: **Don't emit continuation markers in the formatted output.** If the value is `hello` (achieved via continuation), emit `hello`. If the value is empty (continuation to nothing), emit nothing. The formatter's job is to produce CANONICAL output, not to preserve continuation syntax.

This is the architecturally correct fix: **the printer should emit decoded values, not raw continuations.** Continuations are an input syntax convenience, not a formatting requirement. No one expects `cfv format` to KEEP continuation lines — they expect normalized output.

**Revised fix**: In the printer, when emitting a value, decode continuations and emit the decoded content:

```go
// Instead of: buf.Write(e.value.Raw)
// Do: buf.Write(decodeValue(e.value.Raw))
```

Where `decodeValue` resolves `\` + newline + leading whitespace into nothing (collapses continuations). This matches what `magiconair/properties` decodes.

But wait — preserving continuations was a deliberate design choice for long values. The plan says "preserving original representation." If someone has:
```
long.url = \
    https://very-long-url.example.com/path
```
We'd collapse it to one line. Is that acceptable?

Looking at what prettier does for Properties: prettier doesn't format `.properties` files. But the conventional behavior of properties formatters is to normalize to single-line values. The continuation syntax exists for human readability in source, but a formatter should produce canonical output.

**Decision**: Collapse continuations in the formatted output. This eliminates the entire class of continuation-related bugs and produces canonical output. If the value is long, it's one long line — same as JSON formatters produce long strings on one line.

**Implementation**:
```go
func decodeValueRaw(raw []byte) []byte {
    var result []byte
    i := 0
    for i < len(raw) {
        if raw[i] == '\\' && i+1 < len(raw) {
            next := raw[i+1]
            if next == '\n' {
                // Continuation: skip \ + newline + leading whitespace
                i += 2
                for i < len(raw) && (raw[i] == ' ' || raw[i] == '\t') {
                    i++
                }
                continue
            }
            if next == '\r' {
                // Continuation: skip \ + CR (+ optional LF) + leading whitespace
                i += 2
                if i < len(raw) && raw[i] == '\n' {
                    i++
                }
                for i < len(raw) && (raw[i] == ' ' || raw[i] == '\t') {
                    i++
                }
                continue
            }
            // Other escape — preserve verbatim
            result = append(result, raw[i], raw[i+1])
            i += 2
        } else {
            result = append(result, raw[i])
            i++
        }
    }
    return result
}
```

In the printer, replace `buf.Write(e.value.Raw)` with `buf.Write(decodeValueRaw(e.value.Raw))`.

**Test**: Fuzz corpus entry passes. Continuation values collapse to single line. Semantic equivalence preserved (decoded value is identical).

**Files**: `pkg/formatter/propfmt/printer.go` (add `decodeValueRaw`, use in value emission)

---

### 11.4: Add semantic equivalence to all FuzzFormatWithOptions targets

Currently, the fuzz targets only check idempotency (format twice = same). They don't check that formatting preserves semantics (parsed value unchanged). Add semantic checking to catch meaning-changing bugs instantly:

**Pattern for each fuzz target**:
```go
f.Fuzz(func(t *testing.T, data []byte, optByte byte) {
    opts := ...
    result, err := fmtr.Format(data, opts)
    if err != nil { return }

    // Idempotency (existing)
    result2, err := fmtr.Format(result, opts)
    if err != nil { t.Fatalf("second format failed: %v", err) }
    if string(result) != string(result2) { t.Fatalf("not idempotent") }

    // Semantic equivalence (NEW)
    if !semanticallyEqual(data, result) {
        t.Fatalf("semantics changed:\n  input:  %q\n  output: %q", data, result)
    }
})
```

**Per-format `semanticallyEqual`**:
- **TOML**: `toml.Unmarshal(a)` == `toml.Unmarshal(b)` (using JSON round-trip for NaN handling, same as stress test)
- **Properties**: `properties.Load(a).Map()` == `properties.Load(b).Map()`
- **INI**: `ini.Load(a)` sections/keys/values == `ini.Load(b)` sections/keys/values
- **XML**: `encoding/xml` decode both, compare trees (same as stress test `xmlEquivalent`)
- **JSONC**: Strip comments, `json.Unmarshal` both, compare
- **YAML**: `yaml.Unmarshal(a)` == `yaml.Unmarshal(b)`

These functions already exist in `stress_format_test.go`. For the fuzz targets (which are in different packages), implement lightweight versions directly in each test file using only the parsing library.

**Files**: 
- `pkg/formatter/tomlfmt/toml_test.go`
- `pkg/formatter/propfmt/properties_test.go`
- `pkg/formatter/inifmt/ini_test.go`
- `pkg/formatter/xmlfmt/xml_test.go`
- `pkg/formatter/jsoncfmt/jsonc_test.go`
- `pkg/formatter/yamlfmt/yaml_test.go`

---

### Tasks

- [ ] 11.1.1: Fix `sortGroups` trailing comment order (emit after entries, not before)
- [ ] 11.1.2: Re-add TOML fuzz corpus entry, verify passes
- [ ] 11.1.3: Add explicit test for multiline string + trailing comment with SortKeys

- [ ] 11.2.1: Investigate INI lexer escape handling in values (does it treat `\` specially?)
- [ ] 11.2.2: Fix the INI sort/key-extraction issue
- [ ] 11.2.3: Re-add INI fuzz corpus entry, verify passes
- [ ] 11.2.4: Add explicit test for backslash keys with SortKeys

- [ ] 11.3.1: Implement `decodeValueRaw` in propfmt/printer.go (collapse continuations)
- [ ] 11.3.2: Replace `buf.Write(e.value.Raw)` with `buf.Write(decodeValueRaw(e.value.Raw))`
- [ ] 11.3.3: Re-add Properties fuzz corpus entry, verify passes
- [ ] 11.3.4: Update existing continuation test fixtures if output changes (single-line values)
- [ ] 11.3.5: Verify semantic equivalence (magiconair parses same values from formatted output)

- [ ] 11.4.1: Add semantic equivalence check to TOML `FuzzFormatWithOptions`
- [ ] 11.4.2: Add semantic equivalence check to Properties `FuzzFormatWithOptions`
- [ ] 11.4.3: Add semantic equivalence check to INI `FuzzFormatWithOptions`
- [ ] 11.4.4: Add semantic equivalence check to XML `FuzzFormatWithOptions`
- [ ] 11.4.5: Add semantic equivalence check to JSONC `FuzzFormatWithOptions`
- [ ] 11.4.6: Add semantic equivalence check to YAML `FuzzYAMLFormatterWithOptions`

- [ ] 11.5: Run all fuzz targets 45s each with semantic checking — fix any new findings
- [ ] 11.6: Run full stress test suite + real-world corpus
- [ ] 11.7: Pipeline verification (vet, fmt, lint, build, test)

## Phase 12: Fix Semantic Edge-Case Bugs (Found by Fuzz Oracle)

**Date**: 2026-07-15
**Source**: Phase 11.4 semantic assertions caught 3 meaning-changing bugs

---

### 12.1: Properties — Form-feed not recognized as separator whitespace

**Input**: `0\f:0` (bytes: `[48, 12, 58, 48]`) — key `0`, form-feed, colon `:`, value `0`
**Expected**: key=`0`, separator=`\f:`, value=`0` (form-feed is whitespace before the colon separator)
**Actual**: key=`0`, no separator found, value=`\f:0` (form-feed treated as start of value)

**Root cause traced**: In `pkg/formatter/propfmt/tokenizer.go`, function `tokenizeKeyValue`:

Line ~111 (separator whitespace before `=`/`:`):
```go
for *pos < len(src) && (src[*pos] == ' ' || src[*pos] == '\t') {
```
Missing `|| src[*pos] == '\f'`. Form-feed is NOT included.

Line ~117 (separator whitespace after `=`/`:`):
```go
for *pos < len(src) && (src[*pos] == ' ' || src[*pos] == '\t') {
```
Same — missing `\f`.

The key scanner (line ~98) correctly breaks on `\f`:
```go
if b == '=' || b == ':' || b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f' {
```
And the top-level leading whitespace (line ~40) correctly handles `\f`:
```go
for pos < len(src) && (src[pos] == ' ' || src[pos] == '\t' || src[pos] == '\f') {
```

Only the separator detection is inconsistent.

**Fix**: Add `|| src[*pos] == '\f'` to BOTH separator whitespace loops in `tokenizeKeyValue`:

Line ~111: `(src[*pos] == ' ' || src[*pos] == '\t' || src[*pos] == '\f')`
Line ~117: `(src[*pos] == ' ' || src[*pos] == '\t' || src[*pos] == '\f')`

**Verification**: After fix, tokenize `0\f:0` produces:
- TokKey: `"0"`
- TokSeparator: `"\f:"` (or `"\f: "` depending on what follows)
- TokValue: `"0"`

Then `decodeValueRaw("0")` = `"0"`. magiconair parses same value. Semantic equivalence preserved.

**Files**: `pkg/formatter/propfmt/tokenizer.go` lines ~111 and ~117

---

### 12.2: INI — Bare CR inside quoted values treated as line terminator

**Input**: `0="\r"\n` (key `0`, quoted value containing bare CR byte)
**ini.v1 parses**: key=`0`, value=`\r` (CR preserved inside quotes)
**Our output**: Error — tokenizer splits at the CR, producing malformed output

**Root cause traced**: The INI tokenizer's value lexing in `pkg/formatter/inifmt/lexer.go` treats `\r` as a line terminator unconditionally, even inside quoted values. When it encounters `\r` during value scanning, it terminates the value token at that point. The closing `"` and everything after are orphaned.

The lexer's line-end detection needs to be aware of quoting context. Inside a quoted value (`"..."` or `'...'`), `\r` is literal content, not a line terminator.

**Fix**: In the INI lexer's value tokenization, when inside quotes, only terminate on the closing quote character. Don't check for `\r` or `\n` as line terminators until the quotes are closed.

Find the value scanning code in `lexer.go`. It should have logic like:
```go
// Scan value content
for *pos < len(src) && src[*pos] != '\n' && src[*pos] != '\r' {
    *pos++
}
```

The fix: if the value starts with `"` or `'`, scan until the matching close quote (handling escaped quotes if the format supports them — INI doesn't have escapes, so just scan to the next matching quote). THEN check for line terminator.

```go
if *pos < len(src) && (src[*pos] == '"' || src[*pos] == '\'') {
    // Quoted value — scan to closing quote, CR/LF inside are content
    quote := src[*pos]
    *pos++ // skip opening quote
    for *pos < len(src) && src[*pos] != quote {
        *pos++
    }
    if *pos < len(src) {
        *pos++ // skip closing quote
    }
    // After closing quote, consume to end of line (anything after close quote on same line)
    for *pos < len(src) && src[*pos] != '\n' && src[*pos] != '\r' {
        *pos++
    }
} else {
    // Unquoted value — scan to line end as before
    for *pos < len(src) && src[*pos] != '\n' && src[*pos] != '\r' {
        *pos++
    }
}
```

Note: ini.v1's `PreserveSurroundedQuote` option means it keeps the quotes as part of the value. Our tokenizer should capture the entire quoted content (including quotes) as the value raw, which the printer emits verbatim.

**Verification**: `0="\r"\n` tokenizes as:
- TokKey: `"0"`
- TokSeparator: `"="`
- TokValue: `""\r""` (the full quoted string including quotes)
- TokNewline: `"\n"`

After format: `0 = "\r"\n` — ini.v1 re-parses as key=`0`, value=`\r`. Semantic equivalence preserved.

**Files**: `pkg/formatter/inifmt/lexer.go` (value scanning section)

---

### 12.3: YAML — FinalNewline=false strips block scalar clip newline

**Input**: `A: |\n  0\n` with `FinalNewline=false` (optByte bit 3)
**yaml.v3 parses**: `{A: "0\n"}` — default clip preserves exactly one trailing newline
**Our output**: `A: |\n  0` (no trailing newline) — yaml.v3 re-parses as `{A: "0"}` (no newline)
**Semantic change**: Value lost its trailing newline (`"0\n"` → `"0"`)

**Root cause traced**: In `printFormatted` (printer.go), after serialization:
```go
if !endsWithKeepChomping(tokens) {
    out = bytes.TrimRight(out, "\r\n")
}
```

`endsWithKeepChomping` only returns true for `|+` (keep chomping — header contains `+`). Default clip (`|`) returns false. So `TrimRight` strips the trailing newline — but that newline IS the clip newline (part of the scalar value).

The only block scalar where it's safe to strip trailing newlines is `|-` (strip chomping). Both `|` (clip) and `|+` (keep) have semantically significant trailing newlines.

**Fix**: Rename `endsWithKeepChomping` to `endsWithBlockScalarPreservingNewlines` and change the logic:

```go
// endsWithBlockScalarPreservingNewlines checks whether the last meaningful token
// is a block scalar whose trailing newlines are semantically significant.
// Only |- (strip) allows removal. Both | (clip) and |+ (keep) need their newlines.
func endsWithBlockScalarPreservingNewlines(tokens []Token) bool {
    for i := len(tokens) - 1; i >= 0; i-- {
        switch tokens[i].Kind {
        case TokNewline, TokIndent, TokSpace:
            continue
        case TokBlockScalar:
            return !blockScalarHasStripChomping(tokens[i].Raw)
        default:
            return false
        }
    }
    return false
}

// blockScalarHasStripChomping checks if a block scalar header contains '-' (strip).
func blockScalarHasStripChomping(raw []byte) bool {
    nlIdx := bytes.IndexByte(raw, '\n')
    if nlIdx < 0 {
        return false
    }
    return bytes.IndexByte(raw[:nlIdx], '-') >= 0
}
```

And update the caller:
```go
if !endsWithBlockScalarPreservingNewlines(tokens) {
    out = bytes.TrimRight(out, "\r\n")
}
```

Delete the old `endsWithKeepChomping` and `blockScalarHasKeepChomping` functions.

**Edge case**: What about `|2-` (explicit indent + strip)? The header is `|2-`. `bytes.IndexByte(header, '-')` finds it → correctly returns true (strip). What about `|-2`? Same. What about `|` followed by a comment `# keep-this`? The `-` in the comment would be a false positive! Need to only check before the comment.

Actually — block scalar header syntax is: `[|>] [indent] [chomping] [comment]`. The chomping indicator is `+` or `-`. The indent indicator is a digit 1-9. Comments start with `#`. So we should only scan the header BEFORE any `#` or space (which precedes a comment):

```go
func blockScalarHasStripChomping(raw []byte) bool {
    nlIdx := bytes.IndexByte(raw, '\n')
    if nlIdx < 0 { return false }
    header := raw[:nlIdx]
    // Only check indicators before comment (space + #)
    for _, b := range header {
        if b == ' ' || b == '\t' || b == '#' {
            break // rest is comment
        }
        if b == '-' {
            return true
        }
    }
    return false
}
```

Same fix for the old `blockScalarHasKeepChomping` — but we're replacing it entirely.

**Verification**: 
- `A: |\n  0\n` with FinalNewline=false → output is `A: |\n  0\n` (clip newline preserved, document still ends with newline because block scalar needs it)
- `A: |-\n  0\n` with FinalNewline=false → output is `A: |-\n  0` (strip = no trailing newline, safe to trim)
- `A: |+\n  0\n\n\n` with FinalNewline=false → output keeps all trailing newlines (keep)

**Files**: `pkg/formatter/yamlfmt/printer.go` (replace `endsWithKeepChomping`/`blockScalarHasKeepChomping` with new functions)

---

### Tasks

- [ ] 12.1.1: Add `|| src[*pos] == '\f'` to both separator whitespace loops in propfmt/tokenizer.go
- [ ] 12.1.2: Add fuzz corpus entry for `0\f:0`, verify semantic equivalence
- [ ] 12.1.3: Run propfmt tests

- [ ] 12.2.1: Fix INI lexer value scanning to handle quoted values (CR/LF inside quotes are content)
- [ ] 12.2.2: Add fuzz corpus entry for `0="\r"\n`, verify semantic equivalence
- [ ] 12.2.3: Run INI tests

- [ ] 12.3.1: Replace `endsWithKeepChomping`/`blockScalarHasKeepChomping` with `endsWithBlockScalarPreservingNewlines`/`blockScalarHasStripChomping`
- [ ] 12.3.2: Add fuzz corpus entry for `A: |\n  0\n` with FinalNewline=false, verify semantic equivalence
- [ ] 12.3.3: Run YAML tests
- [ ] 12.3.4: Verify existing |+ tests still pass (behavior unchanged for keep)

- [ ] 12.4: Run all fuzz targets 30s with semantic checking — verify these specific bugs are fixed
- [ ] 12.5: Run stress tests + real-world corpus
- [ ] 12.6: Pipeline verification (vet, fmt, lint, build, test)

---

## v3.1 Roadmap: Configurable Lint Rules

**Not in scope for 3.0.** Captured here for future planning.

**Pitch**: Opinionated style rules across all 18 formats, configured in `.cfv.toml`. Replaces yamllint, dotenv-linter, and format-specific style tools with one unified rule system.

**Command**: `cfv lint .` (separate from `check` which is syntax/schema, and `format` which is formatting)

**Rule categories**:
- Universal: max-line-length, final-newline, no-trailing-whitespace, no-bom, line-endings
- Key/value: no-duplicate-keys, key-naming-convention, key-ordering
- YAML: require-document-start, max-nesting-depth, no-anchors, truthy-style
- TOML: inline-table-max-length, array-style-threshold
- ENV/Properties: key-must-be-uppercase, no-empty-values

**Architecture**: Rules are assertions on the parsed structure (tokens + AST). Same pipeline as format — parse once, check many rules. Each rule is a function `func(tokens, ast, config) []Diagnostic`.

**Config example**:
```toml
[lint]
max-line-length = 120
key-naming = "kebab-case"

[lint.yaml]
require-document-start = true
truthy = "strict"  # only true/false, not yes/no

[lint.env]
key-naming = "UPPER_SNAKE_CASE"
```

**Output**: Same reporters as check/format (stdout, JSON, JUnit, SARIF, GitHub annotations).

