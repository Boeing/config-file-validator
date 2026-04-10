# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
