# Formatter Parity Specification

Target: byte-for-byte match with **taplo** (TOML) and **prettier** (JSON/JSONC/YAML) when both tools use default options and no project-specific config.

This document defines the exact formatting rules cfv must implement to achieve drop-in parity. Each rule is derived from source code analysis and behavioral testing, not guesswork.

---

## TOML — Reference: taplo (defaults)

Source: `tamasfe/taplo` `crates/taplo/src/formatter/mod.rs` `impl Default for Options`

### Taplo Default Options

| Option | Default | Description | cfv Status |
|--------|---------|-------------|:----------:|
| `align_entries` | `false` | Don't vertically align `=` signs | ✅ Match |
| `align_comments` | `true` | Align consecutive inline comments to same column | ❌ Gap (#586) |
| `align_single_comments` | `true` | Apply comment alignment even for single lines | ❌ Gap (#586) |
| `array_trailing_comma` | `true` | Add trailing comma in multiline arrays | ✅ Match |
| `array_auto_expand` | `true` | Expand arrays to multiline when > `column_width` | ⚠️ Partial: inline-table exception (#631) |
| `array_auto_collapse` | `true` | Collapse arrays to one line when ≤ `column_width` | ❌ Gap (#569) |
| `compact_arrays` | `true` | No spaces inside `[` `]` on single-line arrays | ✅ Match |
| `compact_inline_tables` | `false` | Spaces inside `{ }` for inline tables | ❌ Gap (#587) |
| `compact_entries` | `false` | Spaces around `=` | ✅ Match |
| `column_width` | `80` | Line width threshold for array expand/collapse | ⚠️ Partial: inline-table exception (#631) |
| `indent_tables` | `false` | Don't indent `[table]` keys | ✅ Match |
| `indent_entries` | `false` | Don't indent values under tables (zero-indent) | ✅ Match |
| `inline_table_expand` | `true` | Expand inline tables when > `column_width` | ⚠️ Intentional divergence (#631) |
| `indent_string` | `"  "` (2 spaces) | Indent unit | ✅ Match |
| `trailing_newline` | `true` | File ends with newline | ✅ Match |
| `allowed_blank_lines` | `2` | Max consecutive blank lines preserved | ⚠️ Partial |
| `reorder_keys` | `false` | Don't reorder keys | ✅ Match |
| `reorder_arrays` | `false` | Don't reorder array values | ✅ Match |
| `reorder_inline_tables` | `false` | Don't reorder inline table values | ✅ Match |
| `crlf` | `false` | Use LF line endings | ✅ Match |

### Taplo Behavioral Rules (not captured by options alone)

| Rule | Description | cfv Status |
|------|-------------|:----------:|
| T-BL1 | Insert exactly one blank line before each `[table]` and `[[array-of-tables]]` header (except the first in the file) | ❌ Gap (#583) |
| T-BL2 | Preserve blank lines within a table's entries (up to `allowed_blank_lines`) | ✅ Match |
| T-IT1 | Inline tables: space after `{`, before `}`, around `=`, after `,` | ❌ Gap (#587) |
| T-IT2 | Empty inline tables: `{}` (no spaces) | ✅ Match |
| T-AC1 | Array auto-collapse: if formatted array fits in `column_width` on one line, collapse it | ❌ Gap (#569) |
| T-AC2 | Array auto-expand: if formatted array exceeds `column_width`, expand to one item per line | ⚠️ Partial: inline-table exception (#631) |
| T-AC3 | Arrays containing comments are never collapsed | ✅ Match |
| T-TC1 | Multiline arrays have trailing comma on last element | ✅ Match |
| T-CA1 | Consecutive inline comments: align `#` to same column | ❌ Gap (#586) |
| T-CA2 | Single inline comment: preserve padding (align to column if `align_single_comments` is true) | ❌ Gap (#586) |
| T-NL1 | File always ends with exactly one newline | ✅ Match |

### Intentional cfv Deviations

| Rule | Description | Rationale |
|------|-------------|-----------|
| C-IT1 | Each comment-free array nested in an inline table stays single-line regardless of `column_width` or source layout, including when a sibling or parent array contains comments. Arrays containing comments remain multiline. | Required by #631; comment line breaks must be preserved so `#` does not consume later values. |

---

## JSON — Reference: prettier (parser: `json`, defaults)

Source: prettier uses its **JavaScript/estree printer** for `json` parser. The JS printer applies `group()` with `printWidth` to decide inline vs expanded.

### Prettier JSON Defaults

| Option | Default | Description | cfv Status |
|--------|---------|-------------|:----------:|
| `printWidth` | `80` | Line width threshold for collapsing | ❌ Gap (#569) |
| `tabWidth` | `2` | Indent size | ✅ Match |
| `useTabs` | `false` | Normalize tabs to spaces | ❌ Gap (#584) |
| `trailingComma` | N/A for JSON | JSON spec forbids trailing commas | ✅ Match |
| `singleQuote` | N/A for JSON | JSON requires double quotes | ✅ Match |
| `bracketSpacing` | `true` | Space inside `{ }` for **single-line** objects | ❌ Gap (partial) |

### Prettier JSON Behavioral Rules

| Rule | Description | cfv Status |
|------|-------------|:----------:|
| J-COL1 | If an array fits within `printWidth` (accounting for indent), keep it on one line: `[1, 2, 3]` | ❌ Gap (#569) |
| J-COL2 | If an array exceeds `printWidth`, expand to one element per line with trailing indent | ❌ Gap (#569) |
| J-COL3 | If an object fits within `printWidth`, keep it on one line: `{ "a": 1, "b": 2 }` | ❌ Gap (#569) |
| J-COL4 | If an object exceeds `printWidth`, expand to one property per line | ❌ Gap (#569) |
| J-COL5 | Collapsing is recursive: if a child expands, the parent also expands | ❌ Gap (#569) |
| J-COL6 | Single-line objects have spaces inside braces: `{ "key": "value" }` | ❌ Gap (#569) |
| J-COL7 | Single-line arrays do NOT have spaces inside brackets: `[1, 2, 3]` | ✅ Match |
| J-TAB1 | Tab indentation normalized to spaces (using `tabWidth`) | ❌ Gap (#584) |
| J-BL1 | Blank lines before closing `}` or `]` are removed | ❌ Gap (#581) |
| J-BL2 | Blank lines between properties are PRESERVED (one max) | ❌ Gap (#588) |
| J-NL1 | File ends with exactly one newline | ✅ Match |
| J-ORD1 | Key order is preserved (never reordered) | ✅ Match (#566) |

### Prettier `json-stringify` Parser (package.json, package-lock.json, composer.json)

| Rule | Description | cfv Status |
|------|-------------|:----------:|
| JS-EXP1 | Arrays are ALWAYS expanded (one element per line) — never collapsed | ⚠️ Low priority |
| JS-EXP2 | Objects are ALWAYS expanded (one property per line) — never collapsed | ⚠️ Low priority |
| JS-IND1 | Standard 2-space indent | ✅ Match |
| JS-NL1 | File ends with exactly one newline | ✅ Match |

**Note:** cfv does not currently distinguish package.json from other .json files. Prettier uses `json-stringify` for package.json (always-expand) and `json` for everything else (collapse-when-fits). This distinction matters for parity.

---

## YAML — Reference: prettier (defaults)

Source: `prettier/prettier` `src/language-yaml/printer-yaml.js` + `print/*.js`

### Prettier YAML Defaults

| Option | Default | Description | cfv Status |
|--------|---------|-------------|:----------:|
| `tabWidth` | `2` | Indent size | ✅ Match |
| `printWidth` | `80` | Line width (controls flow → block expansion) | ❌ Gap (#569) |
| `singleQuote` | `false` | Prefer double quotes for quoted scalars | ❌ Gap (#580) |
| `bracketSpacing` | `true` | Space inside `{ }` for flow mappings | ❌ Gap (#585) |
| `proseWrap` | `"preserve"` | Don't re-wrap prose content | ✅ Match |
| `trailingComma` | `"all"` | Trailing comma in multiline flow collections | ⚠️ Low priority |

### Prettier YAML Behavioral Rules

**Scalars and Quoting:**

| Rule | Description | cfv Status |
|------|-------------|:----------:|
| Y-Q1 | Unquoted scalars stay unquoted (never adds quotes to bare scalars) | ✅ Match |
| Y-Q2 | Quoted scalars normalize to preferred quote style (`singleQuote` option) | ❌ Gap (#580) |
| Y-Q3 | Default preferred quote is `"double"` (`singleQuote: false`) | ❌ Gap (#580) |
| Y-Q4 | If preferred quote char exists in the value, use the OTHER quote style | ❌ Gap (#580) |
| Y-Q5 | Strings with backslash escapes: keep original quote style | ✅ Match (preserve) |

**Sequences:**

| Rule | Description | cfv Status |
|------|-------------|:----------:|
| Y-S1 | Sequence items under a mapping key are indented by `tabWidth`: `key:\n  - item` | ❌ Gap (#582) |
| Y-S2 | Root-level sequence items are NOT indented: `- item` at column 0 | ✅ Match |
| Y-S3 | Sequence items use `- ` prefix with content aligned 2 spaces from the dash | ✅ Match |

**Flow Collections:**

| Rule | Description | cfv Status |
|------|-------------|:----------:|
| Y-F1 | Flow mappings (default `bracketSpacing: true`): `{ key: value, key2: value2 }` | ❌ Gap (#585) |
| Y-F2 | Flow sequences: `[1, 2, 3]` (no spaces inside brackets) | ✅ Match |
| Y-F3 | Empty flow collections: `{}`, `[]` (no spaces) | ✅ Match |
| Y-F4 | Flow mappings with `bracketSpacing: false`: `{key: value}` | N/A (non-default) |
| Y-F5 | Space after `:` in flow mappings: prettier PRESERVES original (does not normalize) | ✅ Match |
| Y-F6 | Space after `,` in flow collections: always | ✅ Match |

**Block Structure:**

| Rule | Description | cfv Status |
|------|-------------|:----------:|
| Y-B1 | Mapping items: `key: value` (space after colon) | ✅ Match |
| Y-B2 | Mapping with sequence value: key on its own line, sequence indented below | ❌ Gap (#582) |
| Y-B3 | Blank lines between sibling nodes are preserved (one maximum) | ✅ Match |
| Y-B4 | No trailing blank lines at end of document | ✅ Match |
| Y-B5 | Block scalars (`|`, `>`) preserve their indicator and content | ✅ Match |

**Comments:**

| Rule | Description | cfv Status |
|------|-------------|:----------:|
| Y-C1 | Inline comments: single space before `#` | ✅ Match |
| Y-C2 | Leading comments above a node: preserved at same indent | ✅ Match |
| Y-C3 | End comments after a block: indented to match the block | ✅ Match |

**Documents:**

| Rule | Description | cfv Status |
|------|-------------|:----------:|
| Y-D1 | `---` marker preserved if present in source | ✅ Match |
| Y-D2 | `...` marker preserved if present in source | ✅ Match |
| Y-D3 | File ends with exactly one newline | ✅ Match |

---

## Conformance Summary

| Format | Total Rules | ✅ Match | ❌ Gap | ⚠️ Low Priority |
|--------|:-----------:|:--------:|:------:|:---------------:|
| TOML | 30 | 18 | 12 | 0 |
| JSON | 18 | 8 | 8 | 2 |
| YAML | 26 | 19 | 6 | 1 |
| **Total** | **74** | **45 (61%)** | **26 (35%)** | **3 (4%)** |

### Issue Mapping

| Issue | Rules Covered | Impact (files from testing) |
|-------|---------------|:---------------------------:|
| #569 | T-AC1, T-AC2, T-AC3, J-COL1–J-COL6 | 198 |
| #582 | Y-S1, Y-B2 | 63 |
| #580 | Y-Q2, Y-Q3, Y-Q4 | 58 |
| #583 | T-BL1 | 31 |
| #584 | J-TAB1 | 17 |
| #585 | Y-F1 | 10 |
| #586 | T-CA1, T-CA2 | 7 |
| #587 | T-IT1 | 6 |
| #588 | J-BL2 | unknown (not in sample) |
| #581 | J-BL1 | 2 |

---

## Testing Methodology

Parity was measured by formatting files with both tools (cfv and reference tool) using default options and no project config, then comparing output byte-for-byte.

- **TOML**: 586 files across 6 repos (rust-analyzer, next.js, uv, poetry, hugo, ansible)
- **JSON**: 300 files across 12 repos (random sample from 1,625 available)
- **YAML**: 300 files across 12 repos (random sample from 1,138 available)

Repos tested: ansible, argo-cd, compose, hugo, next.js, poetry, prettier, react, ripgrep, rust-analyzer, uv, vscode

### Verification Commands

```bash
# TOML: compare cfv vs taplo
taplo format file.toml
cfv format --fix --no-config --no-editorconfig file.toml

# JSON/YAML: compare cfv vs prettier
prettier --no-config --no-editorconfig file.json
cfv format --fix --no-config --no-editorconfig file.json
```

---

## Resolved Questions

1. **J-BL2**: Prettier PRESERVES blank lines between JSON properties. Only strips blank lines immediately before closing `}` or `]`. → cfv behavior is correct for inter-property blanks; only #581 needed.
2. **JS-EXP1/2**: Yes — prettier uses `json-stringify` for `package.json`, `package-lock.json`, `composer.json` which ALWAYS expands (never collapses). cfv currently has no per-filename parser distinction. → This is a new gap worth a separate issue if needed. Low priority since most package.json files are already expanded.
3. **Y-Q5**: Prettier preserves original quote style when the string contains backslash escapes that are meaningful in `quoteDouble` (e.g. `\n`, `\t`). For simple `\\`, it keeps the original quote. → cfv should match: if string has escape sequences, preserve quote style.
4. **Y-F5**: Prettier does NOT normalize colon spacing in flow mappings. `{a:1}` stays as `{a:1}` (only `bracketSpacing` adds space inside braces). → cfv should NOT add space after `:` if none was present. This means #585 is ONLY about `bracketSpacing` (brace padding), not colon normalization.
5. **YAML trailingComma**: Prettier adds trailing comma in multiline flow collections ONLY when `trailingComma: "all"` (the default). At `printWidth: 80`, most flow collections stay inline so trailing comma rarely triggers. → Low priority; only affects flow collections that break to multiline.
6. **TOML allowed_blank_lines**: Needs verification against cfv's current behavior.
7. **printWidth for YAML flow**: YES. Prettier's `printWidth` controls when flow mappings/sequences break to multiline. At `printWidth: 120` more stays inline; at `printWidth: 40` more expands. → This is part of the general printWidth/collapse feature (#569 scope for YAML flow collections).

## Remaining Open

- Does cfv currently limit consecutive blank lines in TOML? (taplo caps at `allowed_blank_lines: 2`)
- Should cfv implement per-filename parser routing (package.json → always-expand mode)?
