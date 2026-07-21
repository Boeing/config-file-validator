# .cfv.toml `[format]` Design Spec

## Design Decisions

1. **Error on unknown keys** — like biome, yamlfmt, taplo. Not like prettier (silent ignore).
2. **Per-format sections** — `[format.json]`, `[format.yaml]`. Not glob-based overrides.
3. **Same key names everywhere** — `indent` means the same thing in `[format]`, `[format.json]`, and `--indent`. No library leakage.
4. **Resolution: CLI > per-format > global > format-specific defaults** — matches biome's model.
5. **Invalid values are errors** — `indent = "banana"` fails loudly at startup, not silently at format time.
6. **HCL section accepted but all keys ignored** — HCL has one canonical style, but we accept the section so users don't get confusing errors when they have a `[format.hcl]` section.

## Config Schema

```toml
# Global formatting defaults — apply to ALL formats unless overridden.
[format]
indent = 2                    # int, 1-16. Spaces per indent level.
use-tabs = false              # bool. Use tabs instead of spaces.
sort-keys = false             # bool. Sort object/mapping keys alphabetically.
trailing-newline = true       # bool. Ensure file ends with exactly one newline.
line-ending = "lf"            # "lf" | "crlf". Line terminator.
max-line-width = 0            # int, 0=unlimited. Hint for line wrapping.
quote-style = "preserve"      # "double" | "single" | "preserve". Scalar quoting.
trailing-commas = "preserve"  # "all" | "none" | "preserve". Trailing commas on multiline collections.

# Per-format overrides — same keys, override the global [format] values.
[format.json]
sort-keys = true              # JSON defaults to sorted keys
# quote-style has no effect on JSON (spec mandates double quotes)

[format.yaml]
indent = 4                    # Override indent for YAML only
quote-style = "double"        # Force double quotes on YAML scalars

[format.hcl]
# Accepted but all keys ignored — HCL has one canonical style.
# Users won't get errors for having this section.
```

## Key Definitions

| Key | Type | Range | Default | Applies to |
|-----|------|-------|---------|------------|
| `indent` | int | 1–16 | format-specific¹ | all |
| `use-tabs` | bool | — | `false` | json (yaml ignores: spec forbids tabs) |
| `sort-keys` | bool | — | format-specific² | json, yaml |
| `trailing-newline` | bool | — | `true` | all |
| `line-ending` | string | `"lf"`, `"crlf"` | `"lf"` | all |
| `max-line-width` | int | 0–320 | `0` | json |
| `quote-style` | string | `"double"`, `"single"`, `"preserve"` | `"preserve"` | yaml (json always double) |
| `trailing-commas` | string | `"all"`, `"none"`, `"preserve"` | `"preserve"` | jsonc (json forbids them) |

¹ JSON default: 2. YAML default: 2. HCL: ignored (always 2, HashiCorp canonical).
² JSON default: `true` (sorted). YAML default: `false` (preserve order).

**Note**: YAML sort-keys requires implementing a Node tree walker that
reorders `MappingNode.Content` pairs alphabetically. This is part of
this task — the option must work end-to-end, not just exist in config.

## Format-Specific Defaults (hardcoded, not user-visible)

These are the defaults when NO config is provided and NO flags are set:

| | indent | use-tabs | sort-keys | trailing-newline | line-ending | max-line-width |
|---|---|---|---|---|---|---|
| **JSON** | 2 | false | true | true | lf | 0 |
| **YAML** | 2 | false | false | true | lf | 0 |
| **HCL** | (ignored) | (ignored) | (ignored) | (ignored) | (ignored) | (ignored) |

## Resolution Order (highest wins)

```
CLI flag: --indent=4
  ↓ wins over
.cfv.toml [format.json] indent = 3
  ↓ wins over
.cfv.toml [format] indent = 2
  ↓ wins over
.editorconfig (indent_size = 6, if present)
  ↓ wins over
Format-specific hardcoded default (e.g., JSON sort-keys=true)
```

When a value is **not set** at a level, it falls through to the next level.
"Not set" means:
- CLI flag: not provided on the command line
- Config key: absent from the TOML section (key not present at all)
- .editorconfig: key not present or no .editorconfig found
- Default: always present (the bottom of the cascade)

### .editorconfig mapping

| .editorconfig key | cfv option |
|-------------------|------------|
| `indent_style = tab` | `use-tabs = true` |
| `indent_style = space` | `use-tabs = false` |
| `indent_size = N` | `indent = N` |
| `end_of_line = lf` | `line-ending = "lf"` |
| `end_of_line = crlf` | `line-ending = "crlf"` |
| `insert_final_newline = true` | `trailing-newline = true` |
| `insert_final_newline = false` | `trailing-newline = false` |

.editorconfig is resolved per-file (it uses glob sections). A file at
`src/config.yaml` may get different editorconfig values than `test/data.json`.

Disable with `--no-editorconfig` flag or `editorconfig = false` in `.cfv.toml`.

**Important**: `sort-keys = false` in `[format]` IS a setting — it overrides
the JSON default of `true`. Only **absence** falls through.

## CLI Flags

Added to `cfv format`:

```
--indent=N         Override indent width (1-16)
--use-tabs         Use tabs for indentation
--sort-keys        Sort object/mapping keys alphabetically
--no-sort-keys     Disable key sorting (overrides config)
--line-ending=X    Line ending: lf, crlf
--max-line-width=N Max line width hint (0=unlimited)
--trailing-newline=BOOL  Trailing newline (default true)
```

Note: `--sort-keys` / `--no-sort-keys` pair because booleans need both
directions from CLI (you can't "unset" a flag otherwise).

## Edge Cases

1. **`use-tabs = true` in `[format.yaml]`**: silently treated as `false` with default indent. YAML spec forbids tabs. No error — just ignored for YAML.
2. **`indent = 0`**: treated as "use format default" (not an error). Zero means "not set."
3. **`indent = 17`**: error at config load time. Range is 1-16.
4. **`line-ending = "auto"`**: NOT supported (what would it detect from?). Error.
5. **`[format.hcl] indent = 4`**: accepted without error but ignored. The key exists in the schema, it just has no effect for HCL.
6. **`[format.unknown]`**: error. Unknown format names are rejected.
7. **`sort-keys` in `[format]` + absent in `[format.json]`**: the global value applies to JSON. If global says `false`, JSON key sorting is off even though JSON's default is `true`.
8. **`--indent` without `--use-tabs`**: sets indent width, preserves tabs/spaces from config or default.
9. **`--use-tabs` without `--indent`**: enables tabs, indent width irrelevant.
10. **`--use-tabs` + `--indent=4`**: tabs for indentation. Indent width is technically meaningless with tabs but accepted without error (prettier does the same).

## Error Messages

```
cfv: .cfv.toml: [format] unknown key "indnet" (did you mean "indent"?)
cfv: .cfv.toml: [format] indent must be 1-16, got 20
cfv: .cfv.toml: [format] line-ending must be "lf" or "crlf", got "auto"
cfv: .cfv.toml: [format.unknown] unknown format "unknown"
cfv: .cfv.toml: [format] use-tabs must be true or false, got "yes"
```

## Test Matrix

Every cell = one txtar test case. ✓ = must verify correct behavior.

### Key × Source permutations (per key)

| Key | Default only | Global [format] | Per-format [format.json] | CLI flag | CLI > per-format > global |
|-----|---|---|---|---|---|
| indent | ✓ | ✓ | ✓ | ✓ | ✓ |
| use-tabs | ✓ | ✓ | ✓ | ✓ | ✓ |
| sort-keys | ✓ | ✓ | ✓ | ✓ | ✓ |
| trailing-newline | ✓ | ✓ | ✓ | ✓ | ✓ |
| line-ending | ✓ | ✓ | ✓ | ✓ | ✓ |
| max-line-width | ✓ | ✓ | ✓ | ✓ | ✓ |

### Resolution cascade tests

| Scenario | Expected |
|----------|----------|
| No config, no flags | format-specific defaults apply |
| `[format] indent = 4` only | both JSON and YAML get 4-space |
| `[format] indent = 4` + `[format.json] indent = 2` | JSON=2, YAML=4 |
| `[format] sort-keys = false` | JSON loses its default sort-keys=true |
| `[format.json] sort-keys = false` | JSON unsorted, YAML default (false) |
| `--indent=8` + any config | all formats get 8-space |
| `--indent=8` + `[format.json] indent = 2` | JSON=8 (CLI wins) |
| `--sort-keys` + `[format] sort-keys = false` | sort-keys on (CLI wins) |
| `--no-sort-keys` + `[format.json] sort-keys = true` | sort-keys off (CLI wins) |

### Error tests

| Input | Expected error |
|-------|----------------|
| `[format] indnet = 2` | unknown key "indnet" |
| `[format] indent = 0` | indent must be 1-16 |
| `[format] indent = 20` | indent must be 1-16 |
| `[format] indent = "two"` | type error |
| `[format] line-ending = "auto"` | must be "lf" or "crlf" |
| `[format.unknown]` | unknown format |
| `[format] sort-keys = "yes"` | must be true or false |

### Real-world scenario tests

| Scenario | Config |
|----------|--------|
| "I want prettier-like JSON" | `[format.json] indent = 2, sort-keys = false` |
| "I want yamlfmt-like YAML" | `[format.yaml] indent = 2, trailing-newline = true` |
| "Different indent per format" | `[format.json] indent = 2` + `[format.yaml] indent = 4` |
| "Tabs for everything" | `[format] use-tabs = true` |
| "CI consistency" | `[format] line-ending = "lf", trailing-newline = true, sort-keys = true` |

## Internal Wiring

```
cfv.go: parseFormatFlags()
  → reads --indent, --sort-keys, etc.
  → stores as *int / *bool (nil = not set)

cfv.go: applyConfigFile()
  → reads [format] section → configfile.FormatOptions struct
  → reads [format.<type>] → map[string]configfile.FormatOptions

cfv.go: resolveFormatOptions(flagOpts, globalCfg, perFormatCfg, formatName)
  → merges layers: CLI > per-format > global > hardcoded default
  → returns formatter.Options (fully resolved, no nils)

cli.Format(opts)
  → passes resolved Options per-file based on FileType.Name
```

The key change from current code: `cli.Format` currently takes a single
`formatter.Options` for all files. After this task, it needs to resolve
options per-format (JSON gets one Options, YAML gets another).
