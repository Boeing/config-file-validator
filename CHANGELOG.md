# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
