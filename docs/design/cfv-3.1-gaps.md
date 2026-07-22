# cfv 3.1 — Formatting Gaps Plan

## Overview

v3.0 ships with a solid formatting foundation: JSON, YAML, HCL formatters
with config + CLI flags + per-format overrides. v3.1 closes the competitive
gaps identified during the 3.0 review. These are features users will ask for
once they adopt cfv for formatting.

## Gaps (ordered by user impact)

### 1. Quote style control

**Who has it**: prettier (`singleQuote`), yamlfmt (via scalar style config)

**What users want**:
```toml
[format.yaml]
quote-style = "double"    # "double" | "single" | "preserve"

[format.json]
# N/A — JSON spec mandates double quotes
```

**Implementation**: Walk `yaml.Node` tree in `normalizeNode`, change
`ScalarNode.Style` from `SingleQuotedStyle` ↔ `DoubleQuotedStyle` based on
config. Preserve bare/unquoted scalars that don't need quoting.

**Effort**: Small — 1 session. The walker stub already exists.

---

### 2. `--diff` output mode

**Who has it**: gofmt (`-d`), biome (`--write` vs default shows diff), yamlfmt (`-lint`)

**What users want**:
```shell
cfv format --diff .
```

Shows unified diff for each file that would change, without writing.
Useful for code review and CI annotations.

**Implementation**: After `Format()` produces output, if `--diff` is set,
compute a unified diff (stdlib `diff` or `github.com/pmezard/go-difflib`)
and print instead of the `~` status line.

**Effort**: Small-medium — 1-2 sessions. go-difflib is BSD, zero deps.

---

### 3. `.editorconfig` integration

**Who has it**: prettier, most IDEs

**What users want**: Zero config if they already have `.editorconfig`. cfv
reads `indent_size`, `indent_style`, `end_of_line`, `insert_final_newline`
from `.editorconfig` and uses them as defaults (below `.cfv.toml`, above
hardcoded defaults).

**Resolution with editorconfig**:
```
CLI flags > .cfv.toml [format.<type>] > .cfv.toml [format] > .editorconfig > hardcoded defaults
```

**Implementation**: Use `github.com/editorconfig/editorconfig-core-go` (MIT,
actively maintained). Map editorconfig keys to `formatter.Options`:
- `indent_style = tab` → `UseTabs: true`
- `indent_size = 4` → `IndentWidth: 4`
- `end_of_line = lf` → `LineEnding: LF`
- `insert_final_newline = true` → `FinalNewline: true`

**Effort**: Medium — 2 sessions. New dependency, per-file resolution
(editorconfig is glob-based per file).

---

### 4. Trailing comma control (JSONC)

**Who has it**: prettier, biome

**What users want**:
```toml
[format.jsonc]
trailing-commas = "all"   # "all" | "none" | "preserve"
```

**Prerequisite**: JSONC formatter (Phase 3 in the plan). Once that exists,
trailing comma is a `normalizeNode` pass that adds/removes commas.

**Effort**: Medium — 2 sessions (JSONC formatter + trailing comma option).

---

### 5. Intelligent line-width wrapping for YAML

**Who has it**: prettier (wraps flow→block when line exceeds `printWidth`)

**What users want**: Long flow-style lines like `{a: 1, b: 2, c: 3, d: 4}`
auto-expand to block style when they exceed `max-line-width`.

**Implementation**: In `normalizeNode`, check `FlowStyle` mappings/sequences.
Estimate serialized width. If > `MaxLineWidth`, clear the `FlowStyle` flag
so the encoder emits block style.

**Effort**: Medium — 2 sessions. Width estimation is the tricky part.

---

### 6. YAML key sorting

**Who has it**: Nobody standalone (yamlfmt doesn't sort, prettier doesn't sort).
Some users want it for deterministic diffs.

**What users want**:
```toml
[format.yaml]
sort-keys = true
```

**Implementation**: In `normalizeNode`, when processing a `MappingNode`:
sort `Content` pairs (key+value) alphabetically by key `Value`. Must handle
comments correctly — head comments attach to the key they precede.

**Effort**: Medium — 1-2 sessions. Comment attachment during sort is the
hard part.

**Note**: This is being wired into 3.0's config/CLI (the option exists) but
the YAML formatter will initially respect it only for JSON. YAML sort-keys
lands in 3.0 as part of Task 6. If too complex, document as "planned" and
ship in 3.1.

---

## Priority Order for 3.1

```
1. Linting rules engine (cfv lint)    — required for MegaLinter PR
2. JSONC formatter + trailing comma   — completes the JSON family
3. .editorconfig integration          — zero-config adoption path
4. YAML intelligent line-width wrap   — prettier-parity for flow→block
```

## Goal: MegaLinter PR after 3.1

With 3.1 shipped, cfv replaces in MegaLinter:
- jsonlint (syntax) → cfv check
- v8r (schema) → cfv check
- prettier for JSON/YAML (formatting) → cfv format
- yamllint (linting) → cfv lint
- dotenv-linter (ENV linting) → cfv lint

That's 5 tools → 1 binary, with SARIF output none of them have.

### Linting rules engine design (cfv lint)

Architecture:
- `Linter` interface: `Lint(src []byte, config RuleConfig) []Issue`
- Rules are per-format, configurable via `.cfv.toml [lint]` / `[lint.yaml]`
- Per-rule severity: error | warning | off
- Reports through existing reporter pipeline (stdout, JSON, SARIF, JUnit)
- `cfv lint .` = report style issues without modifying files
- `cfv .` (unified command, Phase 4) = check + lint + format in one pass

Required rules for yamllint parity:
- `line-length` (max line width, warning)
- `truthy` (forbid yes/no/on/off, require true/false)
- `comments` (min spaces before inline comment)
- `document-start` (require/forbid ---)
- `document-end` (require/forbid ...)
- `empty-lines` (max consecutive blank lines)
- `indentation` (enforce specific width — already handled by format)
- `key-duplicates` (already handled by check)
- `trailing-spaces` (forbid trailing whitespace)

Optional rules (nice-to-have):
- `quoted-strings` (enforce quote style — overlaps with format quote-style)
- `key-ordering` (alphabetical or custom)
- `anchors` (forbid undeclared, forbid unused)

## Moved to 3.0 (Task 6)

These were originally 3.1 but pulled forward and shipped:

- ✅ YAML sort-keys
- ✅ Quote style control (`quote-style`)
- ✅ `--diff` output mode
- ❌ `.editorconfig` integration — deferred (requires per-file resolution)

## Non-Goals for 3.1

- **Glob-based per-file overrides** (like prettier `overrides`): Our per-format
  sections handle 95% of use cases. Glob overrides add complexity for edge
  cases (different indent for `tsconfig.json` vs other JSON). Defer to 3.2 if
  demanded.
- **Plugin system for custom formatters**: Too early. Wait for community demand.
- **AST-level TOML comment preservation**: Hard. 3.0 ships TOML without
  comments if unstable.Parser proves too complex. Revisit in 3.1.
