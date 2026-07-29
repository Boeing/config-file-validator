---
sidebar_position: 8
---

# Using cfv with Existing Tools

If your project already uses prettier, taplo, or yamlfmt for formatting, cfv reads those configs automatically. No setup required.

```shell
cfv check .
```

This works out of the box because cfv recognizes the config files those tools use and applies the relevant settings when formatting. You get the same formatting behavior without running multiple tools.

## What cfv reads

### `.prettierrc`

Applies to JSON, JSONC, and YAML files.

| Prettier option  | cfv option      | Notes                                       |
|------------------|-----------------|---------------------------------------------|
| `tabWidth`       | Indent width    |                                             |
| `useTabs`        | Spaces or tabs  |                                             |
| `printWidth`     | Max line width  |                                             |
| `endOfLine`      | Line ending     | `lf` and `crlf` map; `auto` and `cr` are ignored |
| `trailingComma`  | Trailing commas | JSONC only; `all`, `es5`, or `none`         |
| `singleQuote`    | Quote style     | YAML only                                   |

Supported config files (checked in this order, first match wins):

1. `.prettierrc` (JSON tried first, then YAML)
2. `.prettierrc.json`
3. `.prettierrc.yaml` / `.prettierrc.yml`
4. `.prettierrc.toml`

cfv walks up from each file's directory and uses the nearest match. Unlike `.editorconfig`, prettier configs are not merged across directory levels.

**Not supported:** `.prettierrc.js`, `.prettierrc.cjs`, `.prettierrc.mjs`, `prettier.config.*` (require JS evaluation), `"prettier"` key in `package.json`, and the `overrides` array.

### `taplo.toml`

Applies to TOML files only.

| Taplo option                      | cfv option      |
|-----------------------------------|-----------------|
| `formatting.indent_string`        | Indent width, or tabs |
| `formatting.column_width`         | Max line width  |
| `formatting.trailing_newline`     | Trailing newline |
| `formatting.reorder_keys`         | Sort keys       |
| `formatting.crlf`                 | Line ending     |
| `formatting.array_trailing_comma` | Trailing commas |

cfv searches for `taplo.toml` or `.taplo.toml` in the current directory and parents. `[[rule]]` sections and options without a cfv equivalent (`align_entries`, `align_comments`, `compact_arrays`, `compact_inline_tables`, `array_auto_expand`, `array_auto_collapse`) are ignored.

### `.yamlfmt`

Applies to YAML files only. Takes precedence over `.prettierrc` for YAML when both exist.

| yamlfmt option              | cfv option     |
|-----------------------------|----------------|
| `formatter.indent`          | Indent width   |
| `formatter.line_ending`     | Line ending    |
| `formatter.max_line_length` | Max line width |

cfv searches for `.yamlfmt` or `.yamlfmt.yaml` walking up from the working directory. Other yamlfmt options (`include_document_start`, `retain_line_breaks`, `pad_line_comments`) are ignored.

### `.editorconfig`

Applies to all formats as a baseline. Format-specific tool configs take precedence over `.editorconfig` for the properties they share.

| EditorConfig property  | cfv option       |
|------------------------|------------------|
| `indent_style`         | Spaces or tabs   |
| `indent_size`          | Indent width     |
| `end_of_line`          | Line ending      |
| `insert_final_newline` | Trailing newline |

## When to add `.cfv.toml`

You don't need `.cfv.toml` if your existing tool configs already describe the formatting you want. cfv reads them and does the right thing.

Add `.cfv.toml` when you want:

- **Format settings in one place.** Instead of settings split across `.prettierrc`, `taplo.toml`, and `.editorconfig`, put everything in `.cfv.toml` and remove the others.
- **cfv-specific options.** Some settings (like `indent-sequences` for YAML or per-format sort-keys) exist only in `.cfv.toml`.
- **Validation options.** Schema maps, exclude directories, and reporter settings go in `.cfv.toml`.

Once `.cfv.toml` exists, cfv stops reading `.prettierrc`, `taplo.toml`, and `.yamlfmt`. It becomes the sole formatting authority. `.editorconfig` support under `.cfv.toml` is planned for v3.1.

## Handling conflicts

If your project has both `.prettierrc` and `.yamlfmt`, there's no conflict. cfv uses `.yamlfmt` for YAML and `.prettierrc` for JSON/JSONC. The two never compete for the same format.

A malformed or unreadable config file is skipped. cfv logs a warning and falls back to defaults for that format.

## Differences from the original tools

cfv aims to produce identical output to prettier (for JSON/JSONC/YAML) and taplo (for TOML) given the same settings. A small number of edge cases produce different output. See [Formatting Differences](../reference/formatting-differences.md) for the full list.
