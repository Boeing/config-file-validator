# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

### Added
- Formatting check functionality with `--check-format` flag
  - CLI flag `--check-format` to enable formatting check of valid config files
  - Currently supports JSON format validation

### Tests
- Comprehensive test coverage for the new functionality
- CLI integration tests for formatting feature including:
  - Valid and invalid JSON file handling

## [1.8.0] - 2024-XX-XX
### Added
- Globbing pattern matching for search paths
- Support for exclude-file-types aliases
- Bypass summary printing for pass-fail groups
- Fuzz testing for validators

### Changed
- Updated to Go 1.25
- Various dependency updates
- Improved error handling and validation

### Fixed
- Various linter warnings and code quality improvements
- Updated coverage badges

## [1.7.0] - Previous Release
### Added
- SARIF reporter support
- Environment variable support for configuration
- HOCON file type support
- Multiple reporter output types

### Changed
- Improved CLI experience
- Updated documentation

## Previous Versions
For versions prior to 1.7.0, please see the git history and release notes on GitHub.
