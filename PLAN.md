# PLAN.md — Stress Test Bug Remediation

**Supersedes**: Task 8 "Final stress test + Opus review" in cfv-3.0-plan.md  
**Date**: 2026-07-07  
**Context**: Adversarial fuzz + edge case testing found 5 bugs across 4 formatters.

## Bug Summary

| # | Formatter | Severity | Bug | Root Cause |
|---|-----------|----------|-----|------------|
| 1 | XML | HIGH | Mixed content breaks idempotency | go-xmlfmt regex can't handle text nodes adjacent to elements |
| 2 | XML | MEDIUM | BOM prefix breaks idempotency | Leading newline not trimmed when BOM precedes it |
| 3 | INI | MEDIUM | Backslash values corrupted on round-trip | ini.v1 treats trailing `\` as line continuation |
| 4 | Properties | MEDIUM | `!` and `#` in keys vanish on round-trip | Library writes unescaped, re-parse treats as comment |
| 5 | HCL | LOW | Returns (nil, nil) on empty input | hclwrite.Format returns nil for empty, no guard |

## Remediation Plan

### Fix 5: HCL empty input — trivial guard (do first)

**Problem**: `Format([]byte{}, opts)` returns `(nil, nil)`.  
**Fix**: After `hclwrite.Format(src)`, if result is nil or empty, return the appropriate output based on `FinalNewline` (either `[]byte("\n")` or `[]byte{}`).

**Files**: `pkg/formatter/hclfmt/hcl.go`  
**Test**: Add test case: `Format([]byte{}, defaultOpts)` returns non-nil `[]byte("\n")`.  
**Risk**: Zero. Pure edge case guard.

---

### Fix 2: XML BOM prefix — subsumed by Fix 1

**Status**: Handled as part of the helium rewrite in Fix 1. BOM is stripped before parse and restored after serialize. No separate fix needed.

---

### Fix 1: XML — replace go-xmlfmt with helium + mixed content skip (SEVERITY: HIGH)

**Problem**: `go-xmlfmt` is regex-based and cannot handle mixed content. Indentation drifts on each format pass.

**Solution**: Replace `go-xmlfmt` with `helium` (already a dependency for XML validation). Use `StripBlanks(true)` on parse + `Format(true).IndentString(indent)` on write. This is idempotent for all structure-only XML (config files).

For true mixed content (text + elements as siblings), detect it and return a sentinel error so the user is notified.

**Implementation**:

1. Define a sentinel error in `pkg/formatter/formatter.go`:
   ```go
   // ErrSkipped indicates the formatter cannot process this file but it's not
   // a syntax error. The file should be reported to the user with the reason
   // but not counted as a failure.
   type ErrSkipped struct {
       Reason string
   }
   func (e *ErrSkipped) Error() string { return "skipped: " + e.Reason }
   ```

2. Rewrite `pkg/formatter/xmlfmt/xml.go`:
   - Parse with `helium.NewParser().StripBlanks(true).Parse(ctx, src)`
   - Before formatting: walk the DOM to detect mixed content. If any element
     has both text children (non-whitespace) AND element children, return
     `&formatter.ErrSkipped{Reason: "contains mixed content"}`
   - Serialize with `helium.NewWriter().Format(true).IndentString(indent)`
   - Handle BOM: strip before parse, restore after serialize

3. Update `pkg/cli/format.go` to handle `*formatter.ErrSkipped`:
   - Use `errors.As` to detect the sentinel
   - Report the file with a distinct message (e.g., `~ path — skipped: reason`)
   - Do NOT count as failure (no exit 1 contribution)
   - Do NOT attempt `--fix`

4. Remove `github.com/go-xmlfmt/xmlfmt` from `go.mod`

**Mixed content detection algorithm**:
```go
func hasMixedContent(node helium.Node) bool {
    // Walk all element nodes. For each element, check if it has
    // both non-whitespace text children AND element children.
    // If so, it's mixed content.
}
```

**Files**:
- `pkg/formatter/formatter.go` (add ErrSkipped)
- `pkg/formatter/xmlfmt/xml.go` (rewrite)
- `pkg/cli/format.go` (handle ErrSkipped)
- `go.mod` (remove go-xmlfmt)

**Test**:
- Existing fixtures still pass (all are structure-only XML)
- Mixed content input returns ErrSkipped (not a panic, not formatted)
- BOM input is idempotent
- `<root><a><b/>0</a></root>` returns ErrSkipped
- Normal XML remains idempotent after rewrite

**Risk**: Medium. Replacing the entire XML formatter engine. But helium is already a dep and the API is straightforward. Main risk is the mixed content detection producing false positives on legitimate config XML.

**Dependency change**: Remove `github.com/go-xmlfmt/xmlfmt`. Net -1 dependency.

---

### Fix 3: INI backslash — disable line continuation

**Problem**: `ini.v1` interprets trailing `\` as a line continuation character. After Write serializes a value containing `\`, re-parsing joins it with the next line.

**Fix**: Set `IgnoreContinuation: true` in `LoadOptions`. This tells the parser to treat `\` as a literal character, not a line continuation. This matches the behavior users expect from a formatter — it should never change the semantic meaning of values.

Verify: after enabling `IgnoreContinuation`, confirm that:
- `0=\\` round-trips correctly (value stays as `\\`)
- Regular values without backslash still work
- Multiline values via Python-style continuation (indented next lines) are NOT affected (those use indentation, not `\`)

**Files**: `pkg/formatter/inifmt/ini.go`  
**Test**: Add test: `[section]\npath = C:\\Users\\test\n` round-trips correctly.  
**Risk**: Low. `IgnoreContinuation` is a documented, stable option. Line continuation in INI files is non-standard and rare. 

---

### Fix 4: Properties — replace library serializer with custom line-oriented formatter

**Problem**: `magiconair/properties` library writes keys containing `!` or `#` without escaping them. On re-parse, those characters are interpreted as comment prefixes. The library's `WriteComment` is not round-trip safe.

**Fix**: Stop using `magiconair/properties` for formatting entirely. Rewrite `propfmt` as a custom line-oriented formatter (same approach as `envfmt`):

1. Use `properties.Load` only for **validation** (confirm the file is parseable)
2. Walk the source lines directly:
   - Comment lines (`#` or `!` prefix) — preserve verbatim
   - Blank lines — preserve
   - Key-value lines — normalize spacing around separator to `key = value`
   - Continuation lines (trailing `\`) — preserve verbatim as part of previous key-value
3. For `SortKeys`: parse keys from lines, sort, re-emit with their attached comments

This eliminates the library's serialization entirely. We only depend on it for the parse-validity check (which the validator already does, but we keep it here for the formatter's contract: return error on invalid input).

**Files**: `pkg/formatter/propfmt/properties.go` (rewrite)  
**Test**:
- `\!=bang` round-trips (key `!` survives)
- `\#=hash` round-trips (key `#` survives)
- Existing fixtures still pass
- Multiline values (trailing `\`) preserved
- Comment preservation

**Risk**: Low. Properties format is simple. The ENV formatter proves this approach works.

---

## Execution Order

```
Fix 5  ✅ HCL empty input guard
Fix 3  ✅ INI IgnoreContinuation (partial — fixed backslash, new quoting issue found)
Fix 4  ✅ Properties custom line-oriented (partial — fixed !/# keys, continuation issue found)
Fix 1  ✅ XML helium rewrite + ErrSkipped + BOM (fully fixed, -1 dep)
```

Fix 2 is subsumed by Fix 1.

---

## Round 2: Remaining Fuzz Failures

### Fix 6: Properties — line continuation handling

**Problem**: Input `0\\\n0` (key "0" with value `\` followed by `0` on the next line). In properties spec, trailing `\` on a value means the next line is a continuation. Our formatter emits `0\ = \n0\n` — but on re-parse, the `\` at end of value joins with the next line `0`, changing the structure.

**Root cause**: `normalizeKeyValue` normalizes spacing on the first line of a multi-line value, but doesn't account for the fact that the value portion may end with a continuation `\`. When we emit `key = value\`, the `\` is now at a different position relative to whitespace, and the second line gets treated differently.

**Fix**: Make the formatter continuation-aware during rendering:

1. In `render()`, when emitting a key-value line, check if the value ends with a continuation backslash (odd count of trailing `\`).
2. If it does, emit the key-value line AND all continuation lines as a single block, preserving the continuation lines verbatim (no normalization on continuation lines — they're part of the value).
3. The `parse()` function already tracks continuation correctly and stores the full multi-line value in `line.value`. The issue is in `render()` where we split back into lines.

**Approach**: Change `render()` to emit `line.value` as-is (which may contain `\n` from continuation). The value was captured verbatim from the source including continuation lines. We only normalize the key and separator on the first line.

```go
// In render, for kindKeyVal:
buf.WriteString(l.key)
buf.WriteString(" = ")
buf.WriteString(l.value)  // value includes continuation lines with \n
buf.WriteByte('\n')
```

The bug is that `l.value` already contains the continuation content (`\n0`), but we're adding an extra `\n` between the `= ` and the start of value. Wait — let me trace it more carefully:

Input: `0\\\n0`
- Line 1: `0\\` — key=`0`, separator=`\`, value=`\` (trailing `\` = continuation)
- Line 2: `0` — continuation of previous value

After parse: `line{key: "0", value: "\\\n0"}` (value is `\` + newline + `0`)

After render: `0 = \\\n0\n`

On re-parse of that output:
- Line 1: `0 = \\` — key=`0`, value=`\\` — but wait, `\\` is two backslashes, an EVEN count, so NOT a continuation. But the original had one backslash (continuation).

**Actual root cause**: The properties spec says `\` followed by `\` is an escaped backslash (literal `\`), while a single trailing `\` is continuation. The input `0\\` has TWO backslashes — that's a literal backslash as the value, NOT a continuation. But `properties.Load` parses it as key=`0`, value=`\` (interpreting `\\` as escaped backslash). Our line-oriented formatter doesn't interpret escapes — it sees `0\\` and with `endsWithContinuation` (odd count check) it sees 2 backslashes = even = no continuation. Then it emits as a single line.

The mismatch: the library's validation pass interprets escape sequences, but our line-oriented formatter preserves them verbatim. When the library says "this is valid" and our formatter says "I'll preserve it as-is", the two can disagree on what the structure means.

**Revised fix**: The line-oriented formatter must NOT normalize lines that the library would interpret differently. The safest approach:

1. Use `properties.Load` for validation (confirms parseable).
2. For formatting: walk lines, normalize ONLY the whitespace around the separator on lines that are clearly simple `key=value` (no continuation, no multi-line).
3. Lines with trailing `\` (odd count): preserve the ENTIRE key-value block verbatim (don't normalize). This is safe because we can't know how the library interprets the escapes without duplicating its logic.

**Implementation**:
- In `parse()`: when we detect a key-value with continuation, store `kind: kindKeyVal` but set a `multiline: true` flag.
- In `render()`: if `multiline`, emit the raw original lines verbatim (no normalization).
- Single-line key-values still get `key = value` normalization.

**Files**: `pkg/formatter/propfmt/properties.go`
**Test**: Fuzz corpus input `0\\\n0` round-trips correctly.
**Risk**: Low. We're being more conservative (less normalization) which is always safer.

---

### Fix 7: INI — custom Write replacing ini.v1's WriteTo

**Problem**: `ini.v1`'s `WriteTo` applies quoting to keys/values containing special characters (backtick, double-quote, `=`, etc.). The quoting it applies is not correctly re-parsed by its own `Load`. Keys like `` 0`" `` get serialized as `` `0`"` `` which fails to round-trip.

**Root cause**: The library's writer adds quoting that its parser doesn't correctly reverse. This is a library bug we can't fix upstream.

**Fix**: Replace `f.WriteTo(&buf)` / `f.WriteToIndent(&buf, indent)` with a custom emitter. Keep `ini.Load` for validation and structural access (sections, keys, values, comments). Write our own serialization:

```go
func writeINI(f *ini.File, indent string) []byte {
    var buf bytes.Buffer
    for _, section := range f.Sections() {
        // Write section comment if any
        if comment := section.Comment; comment != "" {
            for _, line := range strings.Split(comment, "\n") {
                buf.WriteString(line)
                buf.WriteByte('\n')
            }
        }
        // Write section header (skip DEFAULT section header)
        if section.Name() != ini.DefaultSection {
            buf.WriteString("[" + section.Name() + "]\n")
        }
        // Write keys
        for _, key := range section.Keys() {
            if comment := key.Comment; comment != "" {
                for _, line := range strings.Split(comment, "\n") {
                    buf.WriteString(indent + line)
                    buf.WriteByte('\n')
                }
            }
            buf.WriteString(indent + key.Name() + " = " + key.Value() + "\n")
        }
        buf.WriteByte('\n')  // blank line between sections
    }
    return buf.Bytes()
}
```

Key difference from library's Write: we emit `key.Name()` and `key.Value()` directly — no quoting transformation. The name and value are already the parsed (decoded) forms. Since the input was valid (Load succeeded), re-emitting the decoded form with `=` separator produces valid INI that round-trips.

**Wait — problem**: `key.Name()` returns the decoded key name (without escapes/quotes). If we emit that directly, and the key contained `=` or whitespace, the output won't be parseable. For example, key `a = b` would emit as `a = b = value` — ambiguous.

**Revised approach**: Don't use the library's parsed representation for serialization at all. Go fully line-oriented (like ENV and Properties):

1. `ini.Load` for validation only.
2. Walk source lines directly. Classify as: comment, blank, section header, key-value.
3. Normalize spacing around `=` on key-value lines.
4. Preserve everything else verbatim.

This is the same proven pattern as ENV and Properties. No quoting issues because we never decode/re-encode.

**Implementation**:
```go
func (Formatter) Format(src []byte, opts formatter.Options) ([]byte, error) {
    // Validate with library.
    if _, err := ini.LoadSources(loadOpts, src); err != nil {
        return nil, err
    }
    
    // Line-oriented formatting.
    lines := splitLines(src)
    var buf bytes.Buffer
    inSection := false
    indent := buildIndent(opts)
    
    for _, line := range lines {
        trimmed := strings.TrimSpace(line)
        switch {
        case trimmed == "":
            buf.WriteByte('\n')
        case trimmed[0] == '#' || trimmed[0] == ';':
            // Comment — preserve with section indent.
            if inSection && indent != "" {
                buf.WriteString(indent)
            }
            buf.WriteString(trimmed)
            buf.WriteByte('\n')
        case trimmed[0] == '[':
            // Section header.
            inSection = true
            buf.WriteString(trimmed)
            buf.WriteByte('\n')
        default:
            // Key-value line — normalize spacing around separator.
            normalized := normalizeINIKeyValue(trimmed)
            if inSection && indent != "" {
                buf.WriteString(indent)
            }
            buf.WriteString(normalized)
            buf.WriteByte('\n')
        }
    }
    
    // FinalNewline + LineEnding handling...
}
```

**Files**: `pkg/formatter/inifmt/ini.go` (rewrite)
**Test**: Fuzz corpus input `` 0`"=0 `` round-trips. Existing fixtures still pass.
**Risk**: Low. Proven pattern. Library only used for validation.

---

## Execution Order (Round 2)

```
Fix 6  🔲 Properties continuation handling    — 20 min, low risk
Fix 7  🔲 INI custom line-oriented rewrite    — 30 min, low risk
       🔲 Full pipeline verification
       🔲 Final fuzz re-run (45s each, all formatters)
```

## Acceptance Criteria (Round 2)

- Both fuzz corpus failures pass (exact inputs that triggered bugs)
- Fuzz 45s per formatter with zero failures on ALL 8 formatters
- Full pipeline green (vet, fmt, lint, test, coverage ≥ 90%)
- Existing fixtures unchanged (no behavior regression on well-formed input)

After all fixes: re-run the full fuzz suite (45s per formatter) to verify the fixes don't introduce new issues, then run the full pipeline.

## Acceptance Criteria

- All 5 fuzz corpus failures now pass (the exact inputs that triggered the bugs)
- Fuzz runs for 45s per formatter with zero new failures
- Full pipeline green (vet, fmt, lint, test, coverage ≥ 90%)
- No production behavior changes for well-formed input (existing fixtures still pass)

## Decision Points for Engineer

1. **XML Fix 1**: The "detect and skip" approach means XML files with mixed content won't get formatted. Is that acceptable, or should we find/write a better XML indenter? The alternative is removing the XML formatter entirely until we have a proper library.

2. **INI Fix 3**: Enabling `IgnoreContinuation` means files that intentionally use `\` line continuation will be treated as literal backslashes. This is a behavior change. Is that acceptable? (Line continuation in INI is non-standard and incredibly rare outside of Python's configparser.)

3. **Properties Fix 4**: The post-processing only fixes `!` and `#` at the start of keys. If there are OTHER characters that the library fails to escape, we'll find them via the fuzz test re-run. If more surface, we may need to replace `WriteComment` with a custom serializer.

---

## Bugs Found During Manual Stress Testing (post-implementation)

These are cosmetic/low-severity issues discovered during manual QA. They do NOT
block the release but should be addressed before v3.0.0 final.

### Bug A: TOML comment between sections gets indented

**Severity**: LOW  
**Reproduce**:
```toml
[section_a]
key = "value"

# Section B comment
[section_b]
key2 = "value2"
```
After `cfv format --fix`, the `# Section B comment` gets indented as if it
belongs to `[section_a]`:
```toml
[section_a]
  key = "value"

  # Section B comment
[section_b]
  key2 = "value2"
```
**Impact**: Cosmetic. The comment visually appears to belong to the wrong section.
Idempotent (doesn't drift). File remains valid TOML.  
**Fix**: In the TOML formatter, reset `inSection` when a blank line is encountered.
A blank line before a comment that precedes a section header should reset the
indent context.

### Bug B: Empty files get a newline written on format --fix

**Severity**: LOW  
**Reproduce**: `touch empty.toml && cfv format --fix empty.toml`  
**Result**: File goes from 0 bytes to 1 byte (`\n`).  
**Impact**: Cosmetic. FinalNewline=true is doing its job — adding a final newline.
But on an empty file, that means creating content where none existed.  
**Fix**: Skip formatting entirely when the file is empty (0 bytes). Return src unchanged.
Applies to TOML, INI, Properties, ENV formatters.
