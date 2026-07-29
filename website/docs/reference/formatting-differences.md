---
---

# Formatting Differences

cfv aims to produce the same output as prettier (for JSON, JSONC, YAML) and taplo (for TOML) given equivalent settings. In most cases it does. This page documents the known differences.

These are edge cases that rarely appear in real configuration files. Across 12 real-world repositories (ansible, argo-cd, compose, hugo, next.js, poetry, prettier, react, ripgrep, rust-analyzer, uv, vscode), cfv matches the reference tools on 99%+ of files.

## YAML

### Multiline plain scalar reflow

When a plain scalar (unquoted string) wraps across multiple lines and exceeds the print width, prettier moves the value to the next line and re-wraps at `tabWidth` indent. cfv preserves the original line breaks.

prettier:
```yaml
description:
  The item and key options control which entries
  are handled. Default may change in future releases.
```

cfv:
```yaml
description: The item and key options control which entries
  are handled. Default may change in future releases.
```

Affects long descriptions in Ansible modules and similar files. Safe divergence: the YAML parses identically either way.

### Explicit key style

YAML allows explicit keys with `?`. Prettier simplifies `? key` to `key:` when possible. cfv preserves the explicit style.

prettier:
```yaml
key: value
```

cfv:
```yaml
? key
: value
```

This is a style choice. Both are valid YAML and parse identically.

### Anchor and tag ordering

For nodes with both an anchor and a tag, YAML allows either order. Prettier normalizes to tag-first (`!!str &anchor`). cfv preserves the source order.

prettier:
```yaml
!!str &name value
```

cfv:
```yaml
&name !!str value
```

Both are valid per the YAML spec. The parsed result is identical.

## JSON / JSONC

### Block comments inside inline objects

When an inline object contains a block comment before a key, prettier keeps it on one line. cfv expands to multiline.

prettier:
```jsonc
{ /*comment*/ "key": "value" }
```

cfv:
```jsonc
{
  /*comment*/
  "key": "value"
}
```

This only affects JSONC (JSON doesn't have comments). The parsed result is identical.

## TOML

### Unsupported taplo options

cfv reads most taplo formatting options but does not support:

- `align_entries` (align `=` signs across keys)
- `align_comments` (align inline comments)
- `compact_arrays` / `compact_inline_tables` (force single-line)
- `array_auto_expand` / `array_auto_collapse` (custom width thresholds)
- `[[rule]]` sections (per-path overrides)

If your `taplo.toml` relies on these, cfv output may differ from `taplo fmt` for those specific behaviors. cfv uses its own defaults (expand/collapse at column width 80, no entry alignment).

## General

### Leading blank lines

prettier and taplo both preserve leading blank lines at the start of a file. cfv strips them. This affects no real-world config files in practice.

### `.prettierrc.js` and overrides

cfv cannot read JavaScript-based prettier configs (`.prettierrc.js`, `.prettierrc.cjs`, `prettier.config.js`) because they require a JS runtime. If your project uses one of these, cfv falls back to `.editorconfig` or defaults.

The prettier `overrides` array is also not supported. Only top-level options apply.
