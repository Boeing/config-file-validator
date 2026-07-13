# TOML Formatter — Detailed Implementation Plan

**Date**: 2026-07-13
**Based on**: Analysis of taplo's architecture (1334 lines formatter, 277 lines lexer, ~800 lines parser)
**Goal**: Format every real-world TOML file correctly. No one reaches for taplo because cfv can't handle their file.

---

## Architecture Learnings from taplo

taplo uses a 3-layer architecture:
1. **Lexer** (logos-based) → flat token stream with SyntaxKind per token
2. **Parser** (hand-written recursive descent) → builds Rowan green tree (lossless syntax tree)
3. **Formatter** → walks the syntax tree, collects entries into groups, formats and reorders

Key insight: taplo's lexer is only 277 lines because it uses `logos` (regex-based lexer generator). The hard parts (multiline string boundary detection) are handled by 4 custom lexer functions totaling ~100 lines.

**Our approach mirrors this, adapted for Go:**
1. **Lexer** → flat token stream (every byte accounted for)
2. **Grouper** → classifies tokens into logical groups (entries, table headers, comments)
3. **Printer** → emits groups with formatting applied

We skip the Rowan tree layer. taplo needs it for DOM access, error recovery, and LSP features. We only need formatting — a flat token stream grouped into entries is sufficient.

---

## Token Types (matching taplo's SyntaxKind)

```go
type TokenKind int

const (
    Whitespace     TokenKind = iota // spaces, tabs (not newlines)
    Newline                         // \n or \r\n
    Comment                         // # through end of line
    BareKey                         // [A-Za-z0-9_-]+
    BasicString                     // "..." (including quotes)
    MultiLineBasicString            // """...""" (including quotes)
    LiteralString                   // '...' (including quotes)
    MultiLineLiteralString          // '''...''' (including quotes)
    Integer                         // decimal, hex, oct, bin
    Float                           // including nan, inf
    Bool                            // true, false
    DateTime                        // offset, local, date, time
    Dot                             // .
    Comma                           // ,
    Equals                          // =
    BracketOpen                     // [
    BracketClose                    // ]
    BraceOpen                       // {
    BraceClose                      // }
)
```

Each token stores: `Kind`, `Raw []byte` (exact source bytes), `Offset int`.

---

## Lexer Design

### Simple tokens (single character or regex)
- Whitespace: `[ \t]+`
- Newline: `\n` or `\r\n`
- Comment: `#` through end of line
- BareKey: `[A-Za-z0-9_-]+`
- Punctuation: `.` `,` `=` `[` `]` `{` `}`
- Numbers: detect by leading digit or sign, consume greedily
- Bool: `true` or `false`
- DateTime: detect by digit pattern, consume greedily

### String tokens (the hard part)

**Basic string** (`"`):
- Start at opening `"`
- Scan characters, tracking `\` escape (next char is escaped)
- End at unescaped `"`
- taplo's implementation: 15 lines

**Multiline basic string** (`"""`):
- Start at opening `"""`
- Scan characters, tracking `\` escape
- Track consecutive unescaped `"` count
- End when we've seen 3+ consecutive unescaped `"` AND the next char is not `"`
- Edge case: `"""""` = empty multiline string + one extra `"` in content? No: per TOML spec, up to 2 extra `"` before the closing `"""` are part of the content. So `"""""` = content `""` + closing `"""`. But `""""""` (6) is invalid.
- taplo's implementation: handles this by continuing to accumulate quotes until a non-quote char, then checking count ≤ 5 (3 close + 2 content max)

**Literal string** (`'`):
- Start at opening `'`
- Scan until closing `'` (no escape sequences — `\` is literal)
- taplo's implementation: same structure as basic but no escape tracking

**Multiline literal string** (`'''`):
- Start at opening `'''`
- Track consecutive `'` count
- End when we've seen 3+ consecutive `'` AND next char is not `'`
- Same boundary logic as multiline basic but simpler (no escapes)
- taplo: 48 lines, handles up to 5 consecutive quotes (content `''` + closing `'''`)

### Disambiguation: how to tell `"` from `"""`

At a `"` character:
1. Peek next 2 characters
2. If `"""` → start multiline basic string lexer
3. If `""` followed by non-`"` → that's an empty basic string `""` (value is "")
4. If `"` followed by non-`"` → start basic string lexer

Same logic for `'` vs `'''`.

---

## Grouper Design

The grouper takes the flat token stream and produces structured groups:

```go
type GroupKind int

const (
    GroupComment    GroupKind = iota // standalone comment line(s)
    GroupBlankLine                   // one or more blank lines
    GroupTable                       // [table.key]
    GroupArrayTable                  // [[array.table]]
    GroupEntry                       // key = value (may span multiple lines)
)

type Group struct {
    Kind     GroupKind
    Tokens   []Token    // all tokens in this group
    Key      []Token    // key tokens (for sorting)
    Value    []Token    // value tokens (for entry groups)
    Comment  []Token    // trailing comment if any
}
```

The grouper's job:
1. Collect newlines/whitespace into blank line groups
2. Collect `#` comments into comment groups
3. Detect `[` at line start → table header group
4. Detect `[[` at line start → array table header group
5. Everything else → entry group (key = value, potentially multiline via arrays/inline tables)

**Multiline value detection** (for entries):
An entry's value spans multiple lines when:
- Value starts with `[` and brackets aren't balanced on the same line → multiline array
- Value starts with `{` and braces aren't balanced on the same line → multiline inline table
- Value is a multiline string (token kind tells us this)

---

## Printer Design

The printer walks groups and emits formatted output:

```go
func (p *Printer) Print(groups []Group, opts Options) []byte
```

**Per-group formatting:**

| Group kind | Formatting applied |
|-----------|-------------------|
| Comment | Preserve verbatim, apply current indent |
| BlankLine | Collapse to max 1 blank line (configurable) |
| Table | Emit header, reset indent scope |
| ArrayTable | Same as table |
| Entry | Normalize `key = value` spacing, apply indent |

**Entry formatting:**
- Key tokens: emit verbatim (preserve quoting style)
- Separator: normalize to ` = ` (or `=` if compact)
- Value tokens:
  - Scalars (string, number, bool, datetime): emit verbatim
  - Inline tables: emit verbatim OR normalize spacing inside braces
  - Arrays (single-line): emit verbatim if short, expand if > column_width
  - Arrays (multiline): preserve structure, normalize indent per element
  - Multiline strings: emit verbatim (never modify string content)
- Trailing comment: preserve with single space separator

**SortKeys:**
- Collect consecutive entries (not separated by blank lines or comments) into a sort group
- Sort by key within the group
- Comments attached to an entry (immediately preceding, not separated by blank line) travel with it

---

## Edge Cases and Risks

### 1. Multiline string closing sequence (HIGH RISK)
**Problem**: `"""""` — is this `""` content + `"""` close, or `"""` open + `""` partial?
**TOML spec answer**: A multiline basic string opens with `"""` and closes with the first occurrence of `"""` that isn't preceded by an unescaped `\`. Up to 2 additional `"` before the closing `"""` are content. So `"""""` = opening `"""` + content empty + closing `"""` + trailing `"` (error? or next token?).

Actually re-reading the spec: "Since there is no escaping, there is no way to write a three or more length run of quotes inside a multiline basic string other than at the very end." The spec allows `""""` (1 extra) and `"""""` (2 extra) as closing sequences containing 1 or 2 literal quote characters in the content.

**Mitigation**: Port taplo's exact logic. Their lexer has handled this for years with zero reported bugs. Test with TOML test suite.

### 2. Inline comment after multiline value (MEDIUM RISK)
```toml
key = [
    1,
    2,
] # comment
```
The `# comment` belongs to the entry, not to a following group. The grouper must track bracket depth to know when the entry ends.

**Mitigation**: Track bracket/brace depth in the grouper. Entry ends when depth returns to 0 AND we hit a newline.

### 3. Dotted keys with quoted segments (LOW RISK)
```toml
"a.b"."c.d" = "value"
```
For sorting purposes, we compare key segments. The key here has two segments: `a.b` and `c.d`. We must not split on `.` inside quotes.

**Mitigation**: Key tokens are already lexed correctly (BasicString, BareKey, Dot as separate tokens). Sorting compares the key token sequences.

### 4. Trailing commas in arrays (LOW RISK)
```toml
arr = [1, 2, 3,]  # trailing comma
arr2 = [1, 2, 3]  # no trailing comma
```
Both are valid. We preserve whichever style the user has (don't add or remove trailing commas).

**Mitigation**: Arrays are emitted verbatim by default. Only reformat if expanding/collapsing.

### 5. Mixed Windows/Unix line endings (LOW RISK)
**Mitigation**: Normalize all newline tokens to configured line ending during printing.

### 6. BOM (LOW RISK)
**Mitigation**: Strip BOM before lexing, restore after printing.

### 7. Values that look like keys (MEDIUM RISK)
```toml
key = "value = other"  # the = inside the string is NOT a separator
```
**Mitigation**: The lexer handles this correctly — `"value = other"` is a single BasicString token. The grouper never sees the `=` inside it.

---

## Test Strategy

### Unit tests
- Lexer: table-driven tests for each token kind, including all string variants
- Grouper: verify correct group boundaries for multiline values
- Printer: fixture-based (input → expected output)

### Spec compliance
- Run against the official TOML test suite (https://github.com/toml-lang/toml-test) — but only for valid files (we format, not validate)
- Every valid TOML file in the test suite must format without error and remain valid after formatting

### Real-world corpus
- Format every Cargo.toml from the top 100 crates on crates.io
- Format pyproject.toml from popular Python projects (black, ruff, pytest)
- Format Hugo, Netlify, Deno config files

### Fuzz testing
- Feed arbitrary bytes; if pelletier accepts it (valid TOML), our formatter must produce valid TOML
- Idempotency: Format(Format(x)) == Format(x)
- 45s minimum, target 10M+ executions

### Comparison testing
- Run taplo and our formatter on the same 1000 files
- Diff the outputs. We don't need to match taplo byte-for-byte (different style choices are fine), but both outputs must be valid TOML with identical semantic content

---

## File Structure

```
pkg/formatter/tomlfmt/
├── tokenizer.go         // Lexer: source → []Token
├── tokenizer_test.go    // Token-level tests
├── grouper.go           // Tokens → []Group (logical structure)
├── grouper_test.go      // Group boundary tests
├── printer.go           // Groups → formatted output
├── printer_test.go      // Formatting tests
├── toml.go              // Format() entry point (validate + tokenize + group + print)
├── toml_test.go         // Integration tests, fixtures, fuzz
└── testdata/
    ├── *.input.toml
    ├── *.expected.toml
    └── corpus/          // Real-world TOML files for regression
```

---

## Implementation Order

```
Day 1-2: Lexer (tokenizer.go)
  - All simple tokens
  - Basic and literal strings
  - Multiline strings (port taplo's logic)
  - Fuzz the lexer: every byte accounted for, no panics
  
Day 3: Grouper (grouper.go)
  - Token stream → logical groups
  - Multiline value boundary detection (bracket/brace depth)
  - Comment attachment

Day 4-5: Printer (printer.go)
  - Entry formatting (key = value normalization)
  - Indentation
  - Multiline array formatting
  - Comment preservation

Day 6: SortKeys + Integration
  - Sort entries within groups
  - Wire up Format() entry point
  - Existing fixtures still pass

Day 7-8: Hardening
  - Fuzz (45s all paths)
  - Real-world corpus testing
  - Comparison with taplo output
  - Fix whatever breaks
```

---

## Decisions (resolved 2026-07-13)

1. **Array formatting policy**: Auto-expand at 80 columns, auto-collapse if fits on one line. Matches taplo default.
2. **Inline table expansion**: Expand inline tables when they exceed column width. Matches taplo default.
3. **Alignment**: No vertical alignment. Single space around `=`. Consistent with our other formatters.
4. **Trailing comma policy**: Add trailing commas to multiline arrays. Matches taplo default.
5. **Allowed blank lines**: Max 2 consecutive blank lines. Matches taplo default.

---

## Dependencies

- `pelletier/go-toml/v2` — validation only (Unmarshal to confirm valid TOML before formatting)
- No new dependencies
- TOML test suite (test-time only, not compiled in)
