# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- SARIF syntax and schema validation using the go-sarif library
- `--schema` flag to validate files against their schema (only SARIF supported currently)
- `CFV_SCHEMA` environment variable equivalent for `--schema`
- `SchemaValidator` optional interface for validators that support schema validation
- Fail-fast validation that errors immediately if `--schema` or `--check-format` is requested for a file type that doesn't support it

### Changed

- `FormatValidator` is now an optional interface (no longer embedded in `Validator`), checked via type assertion
- Removed `ErrMethodUnimplemented` and all stub `ValidateFormat` methods
- Refactored `getFlags` to accept args parameter for improved testability
- Extracted `handleDir` and `handleFile` from `findOne` to reduce cyclomatic complexity
- Extracted flag validation functions from `getFlags` to reduce cyclomatic complexity
- Shortened CLI flag descriptions for cleaner `--help` output
- Eliminated all checked-in test fixtures and golden files; tests now use `t.TempDir()` via `internal/testhelper`
- Refactored group output tests into table-driven tests
- Refactored reporter tests to use content-based assertions instead of golden file comparisons

### Fixed

- `--help` flag no longer prints usage twice
- `.gitignore` entry `validator` no longer excludes `pkg/validator/` directory
- Fixed `SarifFileType` variable name (was incorrectly declared as `ToonFileType`)
- Added `SarifFileType` to the `FileTypes` slice so SARIF files are discovered during validation
- Promoted `go-sarif/v3` from indirect to direct dependency in `go.mod`

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
