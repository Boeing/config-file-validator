# Per-Format Configuration Parity Analysis

## What We Have Today (FormatOptions)

```toml
[format]
indent = 2
use-tabs = false
sort-keys = false
trailing-newline = true
line-ending = "lf"
max-line-width = 80
quote-style = "preserve"
```

Per-format overrides: `[format.yaml]`, `[format.xml]`, etc.

## What We Need (by format)

### YAML — competing with: prettier, yamlfmt

| Option | prettier | yamlfmt | We have | Need |
|--------|----------|---------|---------|------|
| indent width | `tabWidth` | `indent` | ✅ `indent` | — |
| single quote | `singleQuote` | `force_quote_style` | ✅ `quote-style` | — |
| prose wrap | `proseWrap` | — | ❌ | Add: `prose-wrap` (always/never/preserve) |
| indent sequences | — | `indentless_arrays` | ❌ | Add: `indent-sequences` (true/false) |
| include doc start | — | `include_document_start` | ❌ | Add: `document-start` (true/false) |
| retain blank lines | — | `retain_line_breaks` | ❌ | Add: `retain-blank-lines` (true/false) |
| pad line comments | — | `pad_line_comments` | ✅ (hardcoded 1 space) | Make configurable? Low priority |
| bracket spacing | `bracketSpacing` | — | ❌ | Add: `bracket-spacing` (for flow collections) |

**Minimum for parity:** `indent-sequences`, `document-start`
**Nice to have:** `prose-wrap`, `retain-blank-lines`, `bracket-spacing`

### TOML — competing with: taplo

| Option | taplo | We have | Need |
|--------|-------|---------|------|
| indent string | `indent_string` | ✅ `indent`/`use-tabs` | — |
| trailing newline | `trailing_newline` | ✅ | — |
| reorder keys | `reorder_keys` | ✅ `sort-keys` | — |
| column width | `column_width` | ✅ `max-line-width` (unused) | Implement |
| compact arrays | `compact_arrays` | ❌ | Add: `compact-arrays` |
| array auto expand | `array_auto_expand` | ❌ | Add: `array-expand` |
| inline table expand | `inline_table_expand` | ❌ | Add: `inline-table-expand` |
| allowed blank lines | `allowed_blank_lines` | ❌ | Add: `max-blank-lines` |
| crlf | `crlf` | ✅ `line-ending` | — |

**Minimum for parity:** `compact-arrays`, `max-blank-lines`
**Nice to have:** `array-expand`, `inline-table-expand`

### XML — competing with: prettier @prettier/plugin-xml

| Option | prettier-xml | We have | Need |
|--------|-------------|---------|------|
| tab width | `tabWidth` | ✅ `indent` | — |
| use tabs | `useTabs` | ✅ `use-tabs` | — |
| self-closing space | `xmlSelfClosingSpace` | ❌ | Add: `self-closing-space` |
| quote attributes | `xmlQuoteAttributes` | ❌ | Add: `attribute-quotes` (single/double/preserve) |
| sort attributes | `xmlSortAttributesByKey` | ❌ | Add: `sort-attributes` |
| whitespace sensitivity | `xmlWhitespaceSensitivity` | ❌ | Add: `whitespace-sensitivity` (strict/ignore/preserve) |
| single attr per line | `singleAttributePerLine` | ❌ | Add: `single-attribute-per-line` |
| bracket same line | `bracketSameLine` | ❌ | Low priority |
| print width (attr wrap) | `printWidth` | ✅ `max-line-width` | Implement for attr wrapping |

**Minimum for parity:** `self-closing-space`, `whitespace-sensitivity`
**Nice to have:** `attribute-quotes`, `sort-attributes`, `single-attribute-per-line`

### JSON/JSONC — competing with: prettier, biome

| Option | prettier | We have | Need |
|--------|----------|---------|------|
| tab width | `tabWidth` | ✅ | — |
| use tabs | `useTabs` | ✅ | — |
| trailing comma (JSONC) | `trailingComma` | ❌ | Add: `trailing-comma` |
| single quote | `singleQuote` | ❌ (not standard for JSON) | Skip |
| bracket spacing | `bracketSpacing` | ❌ | Add: `bracket-spacing` |
| print width | `printWidth` | ✅ `max-line-width` | Implement |

**Minimum for parity:** `trailing-comma` (JSONC only)
**Nice to have:** `bracket-spacing`, implement `max-line-width`

### INI — no established competitor

| Option | Standard | We have | Need |
|--------|----------|---------|------|
| indent | EditorConfig | ✅ | — |
| separator | — | ❌ | Add: `separator` (= or :) |
| section spacing | — | ❌ | Nice to have |

**Minimum:** `separator`

### Properties — no established competitor

| Option | Standard | We have | Need |
|--------|----------|---------|------|
| separator | Spring convention | ❌ | Add: `separator` (=, :, space) |
| sort keys | — | ✅ | — |

**Minimum:** `separator`

### ENV — trivial format, no config needed

Already complete.

### HCL — terraform fmt IS the standard

`terraform fmt` has zero configuration. It's opinionated. We match it. Done.

## Summary: What to Add

### Phase 1 (minimum parity — blocking):

```toml
[format.yaml]
indent-sequences = true    # yamlfmt's indentless_arrays inverse

[format.xml]
self-closing-space = true  # space before />
whitespace-sensitivity = "preserve"  # strict/ignore/preserve

[format.jsonc]
trailing-comma = "none"    # all/none

[format.ini]
separator = "="            # = or :

[format.properties]
separator = "="            # =, :, or " " (space)
```

### Phase 2 (nice to have — competitive):

```toml
[format.yaml]
document-start = false
retain-blank-lines = true
bracket-spacing = true

[format.toml]
compact-arrays = true
max-blank-lines = 1

[format.xml]
attribute-quotes = "preserve"
sort-attributes = false
single-attribute-per-line = false

[format.json]
bracket-spacing = true
```

### Phase 3 (advanced — differentiation):

```toml
[format.yaml]
prose-wrap = "preserve"

[format.toml]
array-expand = true
inline-table-expand = false

[format.xml]
bracket-same-line = false
```

## Implementation Approach

1. Add new fields to `FormatOptions` struct (with `*` pointer for optionality)
2. Add corresponding fields to `formatter.Options` (the resolved config passed to formatters)
3. Each formatter checks and respects the new options
4. Defaults match the competing tool's defaults (so switching to cfv is seamless)
5. CLI flags for the most common ones (`--trailing-comma`, `--self-closing-space`)

## Priority vs XML Formatter

The XML formatter needs `whitespace-sensitivity` and `self-closing-space` to have feature parity. These should be implemented AS PART of the XML work, not deferred.

All other format-specific options are additive — they don't block the "9/9 no bail-outs" goal.
