# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `cfv format` subcommand with `--fix` (rewrite in place) and `--diff` (print unified diff) modes
- Formatting support for 9 formats: JSON, JSONC, YAML, TOML, HCL, XML, INI, Properties, ENV
- AST-driven YAML formatter: indent normalization, inline mapping/sequence spacing, flow collection normalization, and alphabetical key sorting
- CST-based formatters for TOML, Properties, and INI using custom tokenizers that preserve comments and structure
- JSONC formatting via hujson CST (preserves comments and existing trailing-comma style while normalizing whitespace)
- `--indent` flag to override indent width on `cfv format`
- `--sort-keys` flag to sort mapping keys alphabetically on `cfv format`
- `--diff` flag for previewing formatting changes without modifying files
- Per-format configuration in `.cfv.toml` via `[format.<type>]` tables (yaml, json, jsonc, toml, hcl, xml, ini, properties, env)
- Format configuration cascade: CLI flags > per-format config > global `[format]` config > `taplo.toml` > `.editorconfig` > defaults
- `trailing-commas` format option (`preserve` | `all` | `none`) to control trailing commas on multiline JSONC collections
- `.editorconfig` auto-detection for `cfv format`: `indent_style`, `indent_size`, `end_of_line`, and `insert_final_newline` are resolved per file (globs, parent directories, and `root = true` are all respected). Disable with `--no-editorconfig` (closes #562)
- `taplo.toml` / `.taplo.toml` auto-detection for TOML formatting: `indent_string`, `column_width`, `trailing_newline`, `reorder_keys`, `crlf`, and `array_trailing_comma` are mapped onto the equivalent cfv options. Disable with `--no-taplo-config` (closes #564)
- `max-line-width` and `trailing-commas` are now honored by the TOML formatter
- Schema validation support for JSONC files via `$schema`, `--schema-map`, and SchemaStore
- Schema validation support for Properties files via `--schema-map` in `.cfv.toml`
- **cfv 3.0 Phase 1**: Renamed binary from `validator` to `cfv`. This is a breaking change — no compatibility shim ships. Update scripts: `validator .` → `cfv check .`
- `cfv check` subcommand — identical behavior to the v2 `validator` binary
- `cfv version` subcommand
- `cfv help [subcommand]` subcommand
- Running `cfv .` without a subcommand dispatches to `check` (backward-compatible invocation style)
- CUE syntax validation (`.cue`) via [cuelang.org/go](https://cuelang.org/go) parser (closes #462)
- KDL Document Language syntax validation (`.kdl`) via [sblinch/kdl-go](https://github.com/sblinch/kdl-go) (closes #463)
- Documentation website at https://boeing.github.io/config-file-validator
- `--reporter=github` option that emits validation errors as GitHub Actions workflow commands so they appear as inline PR annotations, without requiring the separate `validate-configs-action` wrapper (closes #459)
- `--merge-sarif` and `--merge-sarif-dir` options for appending external SARIF runs to the validator's SARIF report (closes #460)
- `--ignore-file` option for applying gitignore-style patterns from files like `.dockerignore` or `.prettierignore` during file discovery (closes #457)
- Justfile syntax validation (`.just`, `justfile`, `Justfile`, `.justfile`) via embedded justfile parser (`pkg/validator/justfile`)
- Automatic file type detection from GitHub Linguist's `languages.yml` via `go generate`
- ~90 known filenames auto-detected (`.babelrc`, `tsconfig.json`, `Pipfile`, `pom.xml`, `.gitconfig`, etc.)
- SchemaStore now resolves schemas for extensionless known files (`.babelrc`, `.clangd`, etc.)
- JSON and JSONC treated as a family for `--file-types` and `--exclude-file-types`
- `go generate` step in CI pipeline to keep Linguist data fresh
- CI lint check to ensure generated files are committed up to date
- Automated Linguist SHA updates via scheduled GitHub Actions workflow (`linguist.yml`) that checks SHA weekly

### Fixed

- TOML formatting now leaves entries under table headers unindented by default while preserving explicit indentation overrides (closes #558).
- XML files without a DOCTYPE declaration are validated as syntax-only again; `ValidateSyntax` now enables DTD validation only when a DOCTYPE is present, restoring compatibility after upgrading `helium` to v0.5.1's stricter "DTD required" semantics (closes #546)
- Local JSON Schema paths are encoded as file URLs on Windows (closes #550)
- JSONC formatting no longer adds trailing commas to files that do not already use them (closes #559).
- Global `--help` now exits after printing usage instead of running validation on the current directory.
- Update Go and npm dependencies to resolve 22 known vulnerabilities (CVE-2026-25680, CVE-2026-48779, and others).
- TOML files with duplicate keys are now rejected as invalid (closes #504).
- Broken symlinks are reported as validation failures instead of aborting the run (closes #505)
- External consumers of this module (e.g. `validate-configs-action`) can now resolve all dependencies without workarounds. The justfile parser was previously a separate nested module (`github.com/Boeing/go-just`) with a `replace` directive that didn't propagate to downstream `go.mod` files.
- Repeating the same `--reporter` type with different output paths now writes each requested output.
- `--schema-map` now warns instead of silently skipping files whose validators do not support external schema validation.
- `--require-schema --schema-map` now fails when a mapped file's validator does not support external schema validation.
- Unsupported-extension caching no longer mutates the user-provided `ExcludeFileTypes` map during file walks.
- Multiple reporters targeting the same output file now fail during startup instead of silently overwriting a report.
- KnownFiles now take priority over extension matching in the finder, so `tsconfig.json` resolves to JSONC (not JSON)
- Extension exclusion cache no longer prevents known files from being found
- Linguist known files that conflict with dedicated validators are automatically excluded (e.g. `.editorconfig` stays with EditorConfig, not INI)
- `cfv format` no longer sorts JSON/JSONC object keys by default, matching the behavior of prettier, biome, and deno fmt. Original key order is now preserved unless `sort-keys = true` is set in `.cfv.toml` or `--sort-keys` is passed on the CLI.

### Changed

- Refactored grouped standard and JSON output to support any number of `--groupby` levels.
- Directory grouped output now uses slash-normalized directory keys without trailing separators; files in the current directory use an empty directory key.

## [2.2.0] - 2026-04-27

### Added

- Justfile syntax validation (`.just`, `justfile`, `Justfile`, `.justfile`) via embedded justfile parser (`pkg/validator/justfile`)
- `--gitignore` flag to skip files and directories matched by `.gitignore` patterns, including nested `.gitignore` files, `.git/info/exclude`, and global git ignore config. Supported via CLI flag, `CFV_GITIGNORE` env var, and `gitignore = true` in `.cfv.toml`.
- Automatic file type detection from GitHub Linguist's `languages.yml` via `go generate`
- ~90 known filenames auto-detected (`.babelrc`, `tsconfig.json`, `Pipfile`, `pom.xml`, `.gitconfig`, etc.)
- SchemaStore now resolves schemas for extensionless known files (`.babelrc`, `.clangd`, etc.)
- JSON and JSONC treated as a family for `--file-types` and `--exclude-file-types`
- `go generate` step in CI pipeline to keep Linguist data fresh
- CI lint check to ensure generated files are committed up to date

### Fixed

- KnownFiles now take priority over extension matching in the finder, so `tsconfig.json` resolves to JSONC (not JSON)
- Extension exclusion cache no longer prevents known files from being found
- Linguist known files that conflict with dedicated validators are automatically excluded (e.g. `.editorconfig` stays with EditorConfig, not INI)
- `--exclude-file-types` now excludes files by resolved file type, including extensionless known files like `.gitconfig` and `justfile`


### Changed

- Refactored package-level globals (`GroupOutput`, `Quiet`, `RequireSchema`, `NoSchema`, `SchemaMap`, `SchemaStore`) in the `cli` package into fields on the `CLI` struct, improving concurrency safety and enabling per-instance configuration
- **BREAKING:** `--schemastore` is now a boolean flag that enables remote schema lookup using an embedded SchemaStore catalog. Use `--schemastore-path` to point to a local clone.

### Fixed

- `--depth=0` with trailing slash on search path no longer recurses into subdirectories

### Added

- Configuration file support (`.cfv.toml`): auto-discovered in current and parent directories, or specified with `--config`
- `.cfv.toml` is validated against an embedded JSON Schema — typos and invalid values are caught immediately
- `--config` flag to specify a config file path explicitly
- `--no-config` flag to disable automatic config file discovery
- Stdin support: use `-` as the search path with `--file-types` to validate piped input (e.g., `echo '{"key": "value"}' | validator --file-types=json -`)
- Exit code granularity: exit code 0 for success, 1 for validation errors, 2 for runtime/configuration errors
- JSONC file type support (`.jsonc` extension) with full syntax validation including comments and trailing commas
- JSONC schema validation (`$schema` property, `--schema-map`, `--schemastore`)
- `.tf` and `.tfvars` extensions now recognized as HCL files
- Pre-commit hook support (`.pre-commit-hooks.yaml`) with `config-file-validator` and `config-file-validator-full` hooks
- Embedded SchemaStore catalog in the binary for zero-setup schema lookup
- Remote schema fetching: `--schemastore` now fetches schemas over HTTPS without requiring a local clone
- Schema caching: fetched schemas are cached in `~/.cache/cfv/schemas/` (or `$XDG_CACHE_HOME/cfv/schemas/`) with a 24-hour TTL
- `--schemastore-path` flag for air-gapped environments using a local SchemaStore clone (implies `--schemastore`)
- `CFV_SCHEMASTORE_PATH` environment variable for `--schemastore-path`
- Release workflow now updates the embedded SchemaStore catalog before building binaries

## [2.1.0] - 2026-04-09

### Added

- SARIF reporter now includes `region` with `startLine`/`startColumn` for inline PR annotations in GitHub Actions
- `ValidationError` type with optional `Line`/`Column` fields for structured error positions
- Multiple validation errors are now separated across all reporters: each error on its own line (standard), array of errors (JSON), individual result entries (SARIF), newline-separated in failure message (JUnit)
- `SchemaErrors` type to carry individual schema validation error messages
- Validation errors are prefixed with `syntax:` or `schema:` to distinguish error types
- `error-type` groupby option to group output by syntax errors, schema errors, and passed files
- GitHub Action section in README and index referencing `Boeing/validate-configs-action@v2.0.0`

### Fixed

- XSD validation errors now show detailed diagnostics instead of generic "xsd: validation failed"
- XSD error format cleaned up from `(string):5: Schemas validity error : ...` to `line 5: ...`

## [2.0.0] - 2026-04-08

### Added

- SARIF syntax and schema validation using the go-sarif library
- `--type-map` flag to map glob patterns to file types for files without recognized extensions (e.g. `--type-map="**/inventory:ini"`)
- Functional tests for CLI options
- Schema validation for JSON, YAML, TOML, and TOON
- XML schema validation: XSD via `xsi:noNamespaceSchemaLocation` and inline DTD via `<!DOCTYPE>`
- `--schema-map` flag to map glob patterns to schema files (JSON Schema for JSON/YAML/TOML/TOON, XSD for XML)
- `--schemastore` flag for automatic schema lookup using a local SchemaStore clone
- `--no-schema` flag to disable all schema validation (syntax-only mode)

### Changed

- Refactored unit tests
- Using go 1.26

### Removed

- Formatting validation


## [1.11.0] - 2026-03-25

### Added

- `--file-types` flag to include only specified config file types for validation (inverse of `--exclude-file-types`)
- `CFV_FILE_TYPES` environment variable equivalent for `--file-types`

### Changed

- Refactored validator CLI flag parsing to use `flag.NewFlagSet` instead of the global flag package, improving testability and satisfying the revive `deep-exit` lint rule

## [1.10.0] - 2026-02-05

### Fixed

- Escape the error messages strings in the JUnit report to prevent invalid XML

### Added

- Support of well known files for configuration discovery

## [1.9.0] - 2025-11-14

### Added

- JSON formatting functionality with `--check-format` flag
  - CLI flag `--check-format` to enable formatting check of valid config files, only JSON is supported
  - Checks JSON format with consistent 2-space indentation
- Added a new interface method `ValidateFormat` for format validation
- CHANGELOG.md file
- Github action to verify that changelog was changed for each PR
- CODEOWNERS file
- Support for TOON validation

### Changed

- Interface method name change
  - `Validate` interface renamed to `ValidateSyntax`
- Test examples for good JSON config files were updated to have consistent 2-space indentation
- Build instructions for MacOS were updated to default to arm64

### Fixed

- Windows build instructions did not have the correct variable declarations
