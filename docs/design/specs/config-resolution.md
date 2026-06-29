# Configuration Resolution — cfv 3.0

**Status:** Draft  
**Date:** 2026-06-29  
**Scope:** How cfv discovers, parses, validates, and merges configuration from all sources.

---

## 1. Design principles

1. **Single root config.** One `.cfv.toml` per project. No nested configs, no directory-tree merging. Found by walking up from CWD (or the path given to `--config`). Matches biome, eslint flat config, and dprint.

2. **Strict validation.** Unknown keys are errors. A config file validator does not silently accept bad config. Validation uses the embedded JSON Schema (extended for v3 sections).

3. **Explicit over implicit.** No format options leak through CLI flags. Format configuration lives in the config file. The CLI controls *actions* (`--fix`, `--check`), not *style*.

4. **Safe by default.** Unsafe fix rules require explicit opt-in via `[fix] unsafe = true` or `--unsafe`.

---

## 2. Priority resolution

Sources are checked in this order. The first source that provides a value wins.

```
1. CLI flags                               (highest priority)
2. Environment variables (CFV_*)
3. .cfv.toml [format.<filetype>] section   (per-format overrides)
4. .cfv.toml [format] section              (global format defaults)
5. .cfv.toml top-level / [fix] / etc.      (config file values)
6. Hardcoded defaults                      (lowest priority)
```

Levels 3 and 4 apply only to format options. For all other settings, the order collapses to:

```
CLI flag → env var → config file → default
```

### 2.1 Resolution algorithm (pseudocode)

```go
func Resolve(key string, fileType string) any {
    // 1. CLI flag (only if explicitly set by user)
    if cli.IsSet(key) {
        return cli.Get(key)
    }

    // 2. Environment variable
    envKey := toEnvVar(key) // e.g. "format.indent-width" → "CFV_FORMAT_INDENT_WIDTH"
    if v, ok := os.LookupEnv(envKey); ok {
        return parse(v, typeOf(key))
    }

    // 3. Per-format override (format options only)
    if isFormatKey(key) && fileType != "" {
        if v, ok := config.Format[fileType][stripPrefix(key)]; ok {
            return v
        }
    }

    // 4. Global format section (format options only)
    if isFormatKey(key) {
        if v, ok := config.Format[stripPrefix(key)]; ok {
            return v
        }
    }

    // 5. Config file (non-format keys)
    if v, ok := config.Get(key); ok {
        return v
    }

    // 6. Hardcoded default
    return defaults[key]
}
```

### 2.2 Per-format merge semantics

Per-format sections use **shallow merge by key** against the global `[format]` section. A per-format key overrides the same key in `[format]`. Keys not present in the per-format section inherit from `[format]`.

This is NOT full section replacement. Example:

```toml
[format]
indent-width = 2
sort-keys = false

[format.json]
sort-keys = true
```

Resolution for a `.json` file:
- `indent-width` → 2 (inherited from `[format]`)
- `sort-keys` → true (overridden by `[format.json]`)

Resolution for a `.yaml` file:
- `indent-width` → 2 (from `[format]`)
- `sort-keys` → false (from `[format]`)

---

## 3. Config file discovery

```
func Discover(startDir string) (string, error):
    dir = abs(startDir)
    loop:
        candidate = join(dir, ".cfv.toml")
        if exists(candidate):
            return candidate, nil
        parent = filepath.Dir(dir)
        if parent == dir:  // filesystem root
            return "", nil
        dir = parent
```

Stops at the filesystem root. Returns empty string (no error) if no config found — all defaults apply.

The `--config <path>` flag skips discovery and uses the given path directly. If the file does not exist, that is an error.

---

## 4. Complete .cfv.toml schema

### 4.1 Top-level keys

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `exclude-dirs` | `string[]` | `[]` | Subdirectories to skip during file discovery. |
| `exclude-file-types` | `string[]` | `[]` | File types to ignore. Mutually exclusive with `file-types`. |
| `file-types` | `string[]` | `[]` | File types to include (allowlist). Mutually exclusive with `exclude-file-types`. |
| `ignore-files` | `string[]` | `[]` | Paths to gitignore-style pattern files. Missing files silently skipped. |
| `depth` | `integer` | `0` | Max recursion depth. 0 = unlimited. |
| `reporter` | `string[]` | `["standard"]` | Reporter formats. Format: `type` or `type:path`. |
| `groupby` | `string[]` | `[]` | Group output. Values: `filetype`, `directory`, `pass-fail`, `error-type`. Max 3, unique. |
| `quiet` | `boolean` | `false` | Suppress stdout output. |
| `gitignore` | `boolean` | `false` | Skip files matched by `.gitignore` patterns. |
| `require-schema` | `boolean` | `false` | Fail if a file supports schemas but has no schema declared. |
| `no-schema` | `boolean` | `false` | Disable all schema validation. |
| `schemastore` | `boolean` | `false` | Auto-lookup schemas from SchemaStore catalog. |
| `schemastore-path` | `string` | `""` | Local SchemaStore clone path. Implies `schemastore = true`. |
| `globbing` | `boolean` | `false` | Enable glob matching for search paths. |

### 4.2 `[schema-map]` section

```toml
[schema-map]
"<glob-pattern>" = "<schema-path-or-url>"
```

- Keys: glob patterns matched against file paths (relative to project root).
- Values: path to a JSON Schema file (relative to config location) or an HTTP(S) URL.
- Type: `map[string]string`
- `additionalProperties`: string values only.

### 4.3 `[type-map]` section

```toml
[type-map]
"<glob-pattern>" = "<file-type-name>"
```

- Keys: glob patterns.
- Values: registered file type names (e.g. `"json"`, `"yaml"`, `"ini"`).
- Type: `map[string]string`

### 4.4 `[validators]` section

Per-validator syntax options. These affect how the validator parses files, not how it formats them.

| Section | Key | Type | Default | Description |
|---------|-----|------|---------|-------------|
| `[validators.csv]` | `delimiter` | `string` (1–2 chars) | `","` | Field delimiter. `\t` for tab. |
| `[validators.csv]` | `comment` | `string` (1 char) | `""` | Comment prefix character. |
| `[validators.csv]` | `lazy-quotes` | `boolean` | `false` | Relaxed quote handling. |
| `[validators.json]` | `forbid-duplicate-keys` | `boolean` | `false` | Error on duplicate keys. |
| `[validators.ini]` | `forbid-duplicate-keys` | `boolean` | `false` | Error on duplicate keys in same section. |

`additionalProperties: false` on each sub-section and on `[validators]` itself.

### 4.5 `[format]` section — global format defaults

These defaults apply to all file types unless overridden by a `[format.<filetype>]` section.

| Key | Type | Default | Allowed values | Description |
|-----|------|---------|----------------|-------------|
| `indent-style` | `string` | `"space"` | `"space"`, `"tab"` | Indentation character. |
| `indent-width` | `integer` | `2` | `0`–`16` | Spaces per indent level. Ignored when `indent-style = "tab"`. |
| `final-newline` | `boolean` | `true` | — | Ensure file ends with a newline. |
| `line-ending` | `string` | `"lf"` | `"lf"`, `"crlf"`, `"auto"` | Line ending style. `"auto"` preserves existing. |
| `max-line-width` | `integer` | `0` | `0`–`∞` | Max line length. 0 = unlimited. |
| `sort-keys` | `boolean` | `false` | — | Sort object/map keys alphabetically. |

### 4.6 `[format.<filetype>]` sections — per-format overrides

Each section inherits all keys from `[format]` and may override them. Some formats define additional format-specific keys.

#### Allowed keys per format

| Format | Inherits global keys | Additional keys | Notes |
|--------|---------------------|-----------------|-------|
| `json` | All | — | — |
| `yaml` | All | — | — |
| `toml` | All | — | Convention: `indent-width = 0` for tables. |
| `xml` | All | — | — |
| `hcl` | **None** | — | HCL has canonical style (`hclfmt`). No configurable options. Section must be empty or absent. |
| `cue` | All | — | Convention: `indent-style = "tab"`. |
| `env` | `final-newline`, `line-ending` | `space-around-equals`, `key-casing` | — |
| `ini` | `final-newline`, `line-ending` | `space-around-equals` | — |
| `properties` | `final-newline`, `line-ending` | `separator`, `space-around-separator` | — |

#### Format-specific keys

| Section | Key | Type | Default | Allowed values | Description |
|---------|-----|------|---------|----------------|-------------|
| `[format.env]` | `space-around-equals` | `boolean` | `false` | — | `KEY = value` vs `KEY=value`. |
| `[format.env]` | `key-casing` | `string` | `"upper"` | `"upper"`, `"lower"`, `"preserve"` | Enforce key casing convention. |
| `[format.ini]` | `space-around-equals` | `boolean` | `true` | — | `key = value` vs `key=value`. |
| `[format.properties]` | `separator` | `string` | `"="` | `"="`, `":"`, `" "` | Key-value separator. |
| `[format.properties]` | `space-around-separator` | `boolean` | `true` | — | Spaces around the separator character. |

#### Schema validation rules for `[format.<filetype>]`

- `[format.hcl]`: `additionalProperties: false`, `properties: {}`. Any key is an error.
- `[format.env]`: allows global inheritable keys (`final-newline`, `line-ending`) plus `space-around-equals` and `key-casing`.
- `[format.ini]`: allows global inheritable keys plus `space-around-equals`.
- `[format.properties]`: allows global inheritable keys plus `separator` and `space-around-separator`.
- All other `[format.<filetype>]` sections: allow the same keys as `[format]` (all global keys), no additional keys.
- A `[format.<filetype>]` section for an unrecognized filetype is an error.

### 4.7 `[fix]` section

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `unsafe` | `boolean` | `false` | Allow unsafe fix rules. Overridden by `--unsafe` / `--no-unsafe` CLI flags. |
| `exclude-rules` | `string[]` | `[]` | Rule IDs to never apply, even when `--fix` is active. |

---

## 5. Rule ID registry

All fix rules have canonical IDs in the form `category/rule-name`.

### 5.1 Safe rules (applied by default with `--fix`)

| Rule ID | Category | Description |
|---------|----------|-------------|
| `syntax/trailing-comma` | Syntax | Remove trailing commas in formats that forbid them (JSON). |
| `syntax/dangling-comma` | Syntax | Add trailing comma in formats that require them (last item in multiline). |
| `syntax/final-newline` | Syntax | Add missing final newline. |
| `syntax/bom` | Syntax | Remove UTF-8 BOM. |
| `syntax/line-ending` | Syntax | Normalize line endings to configured style. |
| `syntax/trailing-whitespace` | Syntax | Remove trailing whitespace from lines. |
| `schema/string-to-int` | Schema | Convert `"123"` → `123` when schema expects integer. |
| `schema/string-to-bool` | Schema | Convert `"true"` → `true` when schema expects boolean. |
| `schema/string-to-float` | Schema | Convert `"1.5"` → `1.5` when schema expects number. |
| `schema/enum-case` | Schema | Fix enum value casing when unambiguous (one candidate matches case-insensitively). |
| `format/indent` | Format | Re-indent to match configured style and width. |
| `format/spacing` | Format | Normalize spacing (around separators, colons, etc.). |
| `format/sort-keys` | Format | Sort keys alphabetically when `sort-keys = true`. |
| `format/quote-style` | Format | Normalize quote style (format-specific conventions). |
| `format/trailing-comma` | Format | Add/remove trailing commas per format convention. |

### 5.2 Unsafe rules (require `unsafe = true` or `--unsafe`)

| Rule ID | Category | Why unsafe |
|---------|----------|-----------|
| `schema/int-to-string` | Schema | Narrowing conversion — `123` → `"123"` may break consumers expecting a number. |
| `schema/unwrap-array` | Schema | `["value"]` → `"value"` — loses array semantics. |
| `syntax/duplicate-keys` | Syntax | Removes duplicate keys — last-wins semantics may differ from consumer's parser. |
| `format/flow-to-block` | Format | Converts YAML flow style to block — changes AST structure, may break anchors/aliases. |
| `format/collapse-multiline` | Format | Collapses multiline strings to single line — changes whitespace in string values. |

### 5.3 Rule ID constraints

- IDs are lowercase, use `/` as category separator, `-` as word separator.
- Pattern: `^(syntax|schema|format)/[a-z][a-z0-9-]*$`
- Unknown IDs in `exclude-rules` are errors (catches typos).

---

## 6. Environment variable mapping

### 6.1 Existing variables (unchanged)

| Env var | Config key | Type |
|---------|-----------|------|
| `CFV_DEPTH` | `depth` | integer |
| `CFV_EXCLUDE_DIRS` | `exclude-dirs` | comma-separated |
| `CFV_EXCLUDE_FILE_TYPES` | `exclude-file-types` | comma-separated |
| `CFV_FILE_TYPES` | `file-types` | comma-separated |
| `CFV_REPORTER` | `reporter` | comma-separated |
| `CFV_GROUPBY` | `groupby` | comma-separated |
| `CFV_QUIET` | `quiet` | boolean (`true`/`1`/`yes`) |
| `CFV_GITIGNORE` | `gitignore` | boolean |
| `CFV_IGNORE_FILES` | `ignore-files` | comma-separated |
| `CFV_REQUIRE_SCHEMA` | `require-schema` | boolean |
| `CFV_NO_SCHEMA` | `no-schema` | boolean |
| `CFV_SCHEMASTORE` | `schemastore` | boolean |
| `CFV_SCHEMASTORE_PATH` | `schemastore-path` | string |
| `CFV_GLOBBING` | `globbing` | boolean |

### 6.2 New variables (v3)

| Env var | Config key | Type | Notes |
|---------|-----------|------|-------|
| `CFV_FORMAT_INDENT_STYLE` | `format.indent-style` | string | `"space"` or `"tab"` |
| `CFV_FORMAT_INDENT_WIDTH` | `format.indent-width` | integer | |
| `CFV_FORMAT_FINAL_NEWLINE` | `format.final-newline` | boolean | |
| `CFV_FORMAT_LINE_ENDING` | `format.line-ending` | string | `"lf"`, `"crlf"`, `"auto"` |
| `CFV_FORMAT_MAX_LINE_WIDTH` | `format.max-line-width` | integer | |
| `CFV_FORMAT_SORT_KEYS` | `format.sort-keys` | boolean | |
| `CFV_FIX_UNSAFE` | `fix.unsafe` | boolean | |
| `CFV_FIX_EXCLUDE_RULES` | `fix.exclude-rules` | comma-separated | |

### 6.3 Env var naming convention

```
CFV_<SECTION>_<KEY>
```

- Section separator: `_` (not `__`).
- Hyphens in key names become underscores: `indent-style` → `INDENT_STYLE`.
- Per-format sections (`[format.json]`) have NO env var mapping. Env vars set global format values only. Per-format overrides are config-file-only.

### 6.4 Env var parsing rules

| Target type | Parsing |
|-------------|---------|
| `boolean` | `"true"`, `"1"`, `"yes"` → true. `"false"`, `"0"`, `"no"` → false. Anything else → error. Case-insensitive. |
| `integer` | `strconv.Atoi`. Invalid → error. |
| `string` | Used as-is. |
| `string[]` | Split on `,`. Trim whitespace from each element. Empty string → empty slice. |

---

## 7. Schema validation rules

The embedded `schema.json` enforces all constraints at parse time. Validation occurs before any resolution logic — if the config file is invalid, cfv exits with an error.

### 7.1 Structural constraints

| Constraint | Schema mechanism |
|-----------|-----------------|
| Unknown top-level keys | `additionalProperties: false` |
| Unknown keys in `[validators.*]` | `additionalProperties: false` on each sub-object |
| Unknown keys in `[format]` | `additionalProperties: false` (only known global keys) |
| Unknown keys in `[format.<filetype>]` | Per-type schema with `additionalProperties: false` |
| Unknown filetype in `[format.*]` | `propertyNames` restricted to known format names |
| Unknown keys in `[fix]` | `additionalProperties: false` |
| `[format.hcl]` with any keys | `additionalProperties: false`, `properties: {}` |
| `key-casing` outside `[format.env]` | Key only defined in `format.env` schema |
| `separator` outside `[format.properties]`/`[format.ini]` | Key only defined in those schemas |
| Mutual exclusion: `file-types` + `exclude-file-types` | `not: { required: ["file-types", "exclude-file-types"] }` when both non-empty |
| Mutual exclusion: `require-schema` + `no-schema` | `not: { required: ["require-schema", "no-schema"] }` when both true |
| Invalid rule ID in `exclude-rules` | Validated at runtime against the rule registry (not in JSON Schema) |

### 7.2 Type and range constraints

| Key | Constraint |
|-----|-----------|
| `depth` | integer, minimum 0 |
| `indent-width` | integer, minimum 0, maximum 16 |
| `max-line-width` | integer, minimum 0 |
| `indent-style` | enum: `["space", "tab"]` |
| `line-ending` | enum: `["lf", "crlf", "auto"]` |
| `key-casing` | enum: `["upper", "lower", "preserve"]` |
| `separator` | enum: `["=", ":", " "]` |
| `groupby` items | enum: `["filetype", "directory", "pass-fail", "error-type"]` |
| `groupby` | maxItems: 3, uniqueItems: true |
| `delimiter` | string, minLength 1, maxLength 2 |
| `comment` | string, minLength 1, maxLength 1 |

---

## 8. Error messages

Clear, actionable error messages for common misconfigurations.

### 8.1 Unknown key errors

```
error: .cfv.toml: unknown key "indentWidth" in [format]
  hint: did you mean "indent-width"?

error: .cfv.toml: unknown key "space-around-equals" in [format.json]
  hint: this key is only valid in [format.env], [format.ini], [format.properties]

error: .cfv.toml: unknown key "tab-size" in [format]
  hint: use "indent-width" to set the number of spaces per indent level

error: .cfv.toml: [format.hcl] does not accept any keys
  hint: HCL uses canonical formatting with no configurable options
```

### 8.2 Invalid value errors

```
error: .cfv.toml: invalid value for "indent-style": "tabs"
  hint: allowed values are "space" or "tab"

error: .cfv.toml: invalid value for "indent-width": -1
  hint: must be between 0 and 16

error: .cfv.toml: invalid value for "separator": "|"
  hint: allowed values are "=", ":", or " "
```

### 8.3 Mutual exclusion errors

```
error: .cfv.toml: "file-types" and "exclude-file-types" cannot both be set
  hint: use one or the other to control which file types are validated

error: .cfv.toml: "require-schema" and "no-schema" cannot both be true
```

### 8.4 Rule ID errors

```
error: .cfv.toml: unknown rule ID "syntax/trailing_comma" in fix.exclude-rules
  hint: did you mean "syntax/trailing-comma"?

error: .cfv.toml: unknown rule ID "format/indent-width" in fix.exclude-rules
  hint: known format rules: format/indent, format/spacing, format/sort-keys, format/quote-style, format/trailing-comma
```

### 8.5 Environment variable errors

```
error: CFV_FORMAT_INDENT_WIDTH: invalid integer value "two"
error: CFV_FIX_UNSAFE: invalid boolean value "maybe" (expected true/false/1/0/yes/no)
error: CFV_FORMAT_INDENT_STYLE: invalid value "tabs" (expected "space" or "tab")
```

### 8.6 Config discovery errors

```
error: --config path/to/.cfv.toml: file not found
```

---

## 9. CLI flags interaction

### 9.1 Flags that control format/fix behavior

| Flag | Effect | Priority |
|------|--------|----------|
| `--fix` | Enable auto-fix mode. Applies safe rules. | Action flag — not a config key. |
| `--unsafe` | Allow unsafe fix rules. | Overrides `fix.unsafe` and `CFV_FIX_UNSAFE`. |
| `--no-unsafe` | Disallow unsafe rules (explicit opt-out). | Overrides `fix.unsafe` and `CFV_FIX_UNSAFE`. |
| `--config <path>` | Use specific config file (skip discovery). | — |

### 9.2 No CLI flags for format options

There are no flags like `--indent-width`, `--sort-keys`, `--line-ending`, etc.

Rationale: 18 formats × N options per format = combinatorial explosion of flags. Format configuration belongs in the config file. The CLI controls actions (`--fix`, `--check`), not style.

If a user needs a one-off override, they set an env var:

```shell
CFV_FORMAT_INDENT_WIDTH=4 validator --fix .
```

---

## 10. Example configurations

### 10.1 Minimal (validation only, no formatting)

```toml
exclude-dirs = ["node_modules", "vendor"]
gitignore = true
```

### 10.2 Standard project with formatting

```toml
exclude-dirs = ["node_modules", "vendor", "dist"]
gitignore = true
schemastore = true

[format]
indent-style = "space"
indent-width = 2
final-newline = true
line-ending = "lf"

[format.json]
sort-keys = true

[format.toml]
indent-width = 0
```

### 10.3 Monorepo with strict schema enforcement

```toml
exclude-dirs = ["node_modules", ".git", "dist", "build"]
gitignore = true
schemastore = true
require-schema = true

[schema-map]
"**/tsconfig.json" = "https://json.schemastore.org/tsconfig"
"**/package.json" = "https://json.schemastore.org/package"
".github/workflows/*.yml" = "https://json.schemastore.org/github-workflow"

[format]
indent-style = "space"
indent-width = 2
final-newline = true
line-ending = "lf"
sort-keys = false

[fix]
unsafe = false
exclude-rules = ["format/sort-keys"]
```

### 10.4 Go project

```toml
exclude-dirs = ["vendor", "testdata"]
gitignore = true

[type-map]
"**/go.sum" = "text"

[format]
indent-style = "tab"
final-newline = true
line-ending = "lf"

[format.json]
indent-style = "space"
indent-width = 4

[format.yaml]
indent-width = 2
indent-style = "space"
```

### 10.5 CI environment (env var overrides)

```shell
export CFV_QUIET=true
export CFV_REPORTER=junit:results/cfv.xml,sarif:results/cfv.sarif
export CFV_GITIGNORE=true
export CFV_FIX_UNSAFE=false
validator --check .
```

### 10.6 Per-format key-value file styles

```toml
[format]
final-newline = true
line-ending = "lf"

[format.env]
space-around-equals = false
key-casing = "upper"

[format.ini]
space-around-equals = true

[format.properties]
separator = "="
space-around-separator = true
```

---

## 11. Implementation notes

### 11.1 Config struct (Go)

```go
type Config struct {
    // Top-level (existing)
    ExcludeDirs      []string          `toml:"exclude-dirs"`
    ExcludeFileTypes []string          `toml:"exclude-file-types"`
    FileTypes        []string          `toml:"file-types"`
    IgnoreFiles      []string          `toml:"ignore-files"`
    Depth            *int              `toml:"depth"`
    Reporter         []string          `toml:"reporter"`
    GroupBy          []string          `toml:"groupby"`
    Quiet            *bool             `toml:"quiet"`
    Gitignore        *bool             `toml:"gitignore"`
    RequireSchema    *bool             `toml:"require-schema"`
    NoSchema         *bool             `toml:"no-schema"`
    SchemaStore      *bool             `toml:"schemastore"`
    SchemaStorePath  *string           `toml:"schemastore-path"`
    Globbing         *bool             `toml:"globbing"`
    SchemaMap        map[string]string `toml:"schema-map"`
    TypeMap          map[string]string `toml:"type-map"`
    Validators       ValidatorOptions  `toml:"validators"`

    // New in v3
    Format FormatConfig            `toml:"format"`
    Fix    FixConfig               `toml:"fix"`
}

type FormatConfig struct {
    IndentStyle  *string `toml:"indent-style"`
    IndentWidth  *int    `toml:"indent-width"`
    FinalNewline *bool   `toml:"final-newline"`
    LineEnding   *string `toml:"line-ending"`
    MaxLineWidth *int    `toml:"max-line-width"`
    SortKeys     *bool   `toml:"sort-keys"`

    // Per-format overrides (keyed by format name)
    JSON       *FormatJSON       `toml:"json"`
    YAML       *FormatYAML       `toml:"yaml"`
    TOML       *FormatTOML       `toml:"toml"`
    XML        *FormatXML        `toml:"xml"`
    HCL        *FormatHCL        `toml:"hcl"`
    CUE        *FormatCUE        `toml:"cue"`
    Env        *FormatEnv        `toml:"env"`
    INI        *FormatINI        `toml:"ini"`
    Properties *FormatProperties `toml:"properties"`
}

type FormatJSON struct {
    IndentStyle  *string `toml:"indent-style"`
    IndentWidth  *int    `toml:"indent-width"`
    FinalNewline *bool   `toml:"final-newline"`
    LineEnding   *string `toml:"line-ending"`
    MaxLineWidth *int    `toml:"max-line-width"`
    SortKeys     *bool   `toml:"sort-keys"`
}

type FormatEnv struct {
    FinalNewline      *bool   `toml:"final-newline"`
    LineEnding        *string `toml:"line-ending"`
    SpaceAroundEquals *bool   `toml:"space-around-equals"`
    KeyCasing         *string `toml:"key-casing"`
}

type FormatINI struct {
    FinalNewline      *bool   `toml:"final-newline"`
    LineEnding        *string `toml:"line-ending"`
    SpaceAroundEquals *bool   `toml:"space-around-equals"`
}

type FormatProperties struct {
    FinalNewline         *bool   `toml:"final-newline"`
    LineEnding           *string `toml:"line-ending"`
    Separator            *string `toml:"separator"`
    SpaceAroundSeparator *bool   `toml:"space-around-separator"`
}

type FormatHCL struct{} // No fields — empty struct signals "section acknowledged, no options"

type FixConfig struct {
    Unsafe       *bool    `toml:"unsafe"`
    ExcludeRules []string `toml:"exclude-rules"`
}
```

### 11.2 Resolution implementation

The resolver is a standalone function that takes `(Config, CLIFlags, fileType string)` and returns a fully-resolved `ResolvedConfig` with no pointers (all values concrete). This resolved config is computed once per file type at the start of a run, not per-file.

```go
type ResolvedFormatConfig struct {
    IndentStyle  string
    IndentWidth  int
    FinalNewline bool
    LineEnding   string
    MaxLineWidth int
    SortKeys     bool
    // Format-specific (populated based on file type)
    SpaceAroundEquals    bool   // env, ini, properties
    KeyCasing            string // env only
    Separator            string // properties, ini
    SpaceAroundSeparator bool   // properties
}
```

### 11.3 Schema maintenance

The embedded `schema.json` is the single source of truth for config validation. It is hand-maintained (not generated). When adding new config keys:

1. Add the key to `schema.json`.
2. Add the field to the Go struct.
3. Update the resolver to handle the new key.
4. Add tests covering valid and invalid values.

---

## 12. Migration from v2

The v3 config format is a superset of v2. All existing `.cfv.toml` files remain valid. New sections (`[format]`, `[fix]`) are optional and default to safe values.

No migration tool needed. No breaking changes to existing config keys.

---

## 13. Open questions (for design review)

1. Should `[format.<filetype>]` sections for formats that don't support formatting (e.g. `[format.csv]`) be silently ignored or rejected as errors?
2. Should `exclude-rules` accept glob patterns (`format/*`) or only exact rule IDs?
3. Should there be a `[fix.include-rules]` for allowlist-only mode, or is exclude-only sufficient for v3.0?
