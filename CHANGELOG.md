# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [2.0.1] - 2026-04-09

### Added

- SARIF reporter now includes `region` with `startLine`/`startColumn` for inline PR annotations in GitHub Actions

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
