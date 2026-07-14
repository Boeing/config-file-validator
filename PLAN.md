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
| YAML | ⚠️ AST (goccy/go-yaml) — unstable serializer | 1 (stability guard) | **Phase 6A: CST rewrite** |
| JSONC | ✅ CST (hujson + custom walker) | 0 | **Done (dfb2dae)** |
| TOML | ✅ CST (custom tokenizer/grouper/printer) | 0 | **Done (6166dcc)** — identical to taplo |
| XML | ⚠️ DOM (helium) | 2 (ErrSkipped) | Blocked on helium upstream |
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

- [ ] 6A.1: Implement tokenizer in `pkg/formatter/yamlfmt/tokenizer.go`
  - Lex YAML source into flat token stream
  - Handle: indent, comments, block scalars (opaque), flow collections (opaque), quoted strings, plain scalars, sequence entries, mapping pairs, document markers, directives
  - Every byte of input accounted for in exactly one token
  - Fuzz against goccy: if goccy accepts it, our tokenizer must not panic

- [ ] 6A.2: Implement grouper in `pkg/formatter/yamlfmt/grouper.go`
  - Walk token stream, determine structural depth from indent levels
  - Group tokens into entries for SortKeys
  - Track parent indent to detect entry boundaries

- [ ] 6A.3: Implement printer in `pkg/formatter/yamlfmt/printer.go`
  - Walk token stream, replace IndentTokens with normalized indent (depth × width)
  - Handle block scalar content indent shifting
  - Implement SortKeys (reorder entry groups)
  - Apply FinalNewline and LineEnding

- [ ] 6A.4: Replace yaml.go Format function
  - Keep goccy for validation (`Unmarshal`)
  - Replace AST manipulation with: validate → tokenize → group → format → print
  - Remove stability guard
  - Remove `AddColumn`, `reindent`, `normalizeNode`, `sortMappingKeys`, `applyQuoteStyleToValue`
  - No silent bail-outs, no runtime idempotency check

- [ ] 6A.5: Tests
  - All existing fixtures must produce identical output (or better — match prettier where old output was wrong)
  - New fixtures: block scalars, flow collections, anchors/aliases, multi-doc, deeply nested
  - Test against prettier output on real-world files (docker-compose, k8s manifests, GitHub Actions workflows)
  - Fuzz: 45s minimum, zero failures

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
                                                    Byte-for-byte identical to taplo on all test cases.
                                                    Values with comments preserved verbatim (matches taplo).
Phase 2: Properties CST                          ✅ done
                                                    8.7M fuzz executions, zero failures.
                                                    Handles continuations, escaped keys, SortKeys with comments.
Phase 3: INI CST                                 ✅ done
                                                    3.1M fuzz executions, zero failures.
                                                    96.6% coverage. Escaped keys, colon separators, SortKeys within sections.
Phase 6: YAML/ENV cleanup                        ✅ done (ErrSkipped for empty, error for IndentTabs)
                                                    Stability guard still present — will be removed by Phase 6A.
Phase 6A: YAML CST formatter                     — NEXT (custom tokenizer, no goccy serializer)
Phase 5: XML (blocked on helium upstream)        — 1-2 days when unblocked
Phase 7: Ephemeral CLI stress test (ALL formats) — after all formatters done
Phase 8: CLI UX fixes                            — help text + dry-run diff
```

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
