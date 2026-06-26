# AGENTS.md

This file provides context for AI coding agents (Claude Code, Codex, Kiro, etc.) working on the config-file-validator project.

## Project overview

Config File Validator is a Go CLI tool that recursively scans directories for configuration files, detects their format by extension or known filename, and validates syntax and schema. It supports 16 file formats and outputs results in multiple report formats.

- Module path: `github.com/Boeing/config-file-validator/v2`
- Go version: 1.26
- Binary: `cmd/validator/validator.go`
- License: Apache 2.0

## Architecture

```
cmd/validator/       CLI entrypoint, flag parsing, orchestration
pkg/validator/       Validator implementations (one file per format)
pkg/filetype/        FileType registry, extension/known-file mapping
pkg/finder/          Filesystem walker, gitignore support, filtering
pkg/reporter/        Output formatters (stdout, JSON, JUnit, SARIF, GitHub)
pkg/cli/             CLI engine: wires finder → validators → reporters
pkg/schemastore/     SchemaStore catalog lookup and caching
pkg/configfile/      .cfv.toml config file parser
pkg/tools/           Small utility functions
internal/generate/   Code generators (known files from Linguist)
```

## Prerequisites

- Go 1.26+ (see `go.mod`)
- golangci-lint v2 (`go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`)
- Node.js ≥ 20 (only for documentation site)

## Data flow

```
CLI parses flags → builds Finder (with file types, excludes, gitignore)
  → Finder walks filesystem, returns []FileMetadata
  → cli.Run() validates each file (syntax → schema if available)
  → Reporter.Print() formats results to stdout or file
```

The `pkg/cli` package is the orchestrator. It accepts a Finder, validators come from the FileType registry, and reporters are passed in from the CLI entrypoint.

## Local pipeline

When asked to "run the pipeline", "run pre-checks", or "verify before push", run all of these in order. Every check must pass before submitting a PR.

```
go vet ./...
test -z "$(gofmt -s -l -e .)"
golangci-lint run ./...
go generate ./pkg/filetype/...
go build -o /dev/null cmd/validator/validator.go
go test -cover -coverprofile coverage.out ./...
go tool cover -func coverage.out | grep total
```

Coverage must be ≥ 90%.

For fast iteration on a single package, run its tests directly:

```
go test ./pkg/validator/...
go test ./pkg/reporter/...
go test ./pkg/finder/...
```

Save the full pipeline for the final check before pushing.

## Quick reference

```
go test -v -run TestFoo ./pkg/validator/...                          # Run one test
go test -count=1 ./pkg/validator/...                                 # Skip test cache
go build -o ./validator cmd/validator/validator.go && ./validator .   # Build and run locally
go test -bench=. -benchmem ./pkg/finder/...                          # Benchmark finder
```

## Adding a new validator

1. Create `pkg/validator/<format>.go` with a struct implementing `validator.Validator`:

```go
package validator

type FooValidator struct{}

var _ Validator = FooValidator{}

func (FooValidator) ValidateSyntax(b []byte) (bool, error) {
    // Parse b. Return (true, nil) on success or (false, err) on failure.
    // Wrap errors in &ValidationError{Err: ..., Line: ...} when position is available.
}
```

2. Optionally implement `SchemaValidator` and/or `JSONMarshaler` if the format supports schema validation.

3. Register the file type in `pkg/filetype/file_type.go`:
   - Add a package-level `var FooFileType = FileType{...}` with name, extensions, and validator instance.
   - Add the name → pointer entry to `fileTypeRegistry`.
   - Add the value to the `FileTypes` slice in `init()`.

4. Add test cases in `pkg/validator/validator_test.go`. Follow the existing table-driven style. Add fuzz targets if the parser handles untrusted input.

5. Add a test fixture directory if needed under the existing test infrastructure.

6. Run `go generate ./pkg/filetype/...` if the format has known filenames in GitHub Linguist.

7. Update documentation:
   - `website/docs/reference/supported-file-types.md`
   - `website/docs/guides/file-type-detection.md` (if the format has known filenames)
   - `CHANGELOG.md` under `[Unreleased]` → `Added`

## Adding a new reporter

1. Create `pkg/reporter/<name>_reporter.go` implementing the `Reporter` interface:

```go
package reporter

type FooReporter struct {
    outputDest string
    isQuiet    bool
}

func NewFooReporter(outputDest string, isQuiet bool) *FooReporter {
    return &FooReporter{outputDest: outputDest, isQuiet: isQuiet}
}

func (r *FooReporter) Print(reports []Report) error {
    // Format reports and write to stdout or outputDest file.
    // Use outputBytesToFile() for file output.
    // Respect r.isQuiet (suppress stdout when true and outputDest is set).
}
```

2. Wire it into the CLI in `cmd/validator/validator.go`:
   - Add the format name to the `getReporter` switch/map.
   - Update the usage text with the new format name.

3. Add tests in `pkg/reporter/reporter_test.go`.

4. Update documentation:
   - `website/docs/guides/output-reporters.md`
   - `website/docs/reference/cli-flags.md` (update the `--reporter` flag's supported formats list)
   - `CHANGELOG.md` under `[Unreleased]` → `Added`

## PR requirements

Every pull request must:

1. Update `CHANGELOG.md` under the `[Unreleased]` section. Use the appropriate subsection (`Added`, `Fixed`, `Changed`, `Removed`). Follow [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) format.
2. Pass all CI checks (vet, fmt, build, test, coverage ≥ 90%).
3. Pass `golangci-lint run ./...` with zero findings.
4. Use conventional commit messages: `type(scope): description`. Types: `feat`, `fix`, `docs`, `chore`, `ci`, `test`, `refactor`. Scope is optional but encouraged (e.g. `feat(reporter): add TAP output format`).

## Fixing a bug

1. Write a test that reproduces the bug (it should fail before your fix).
2. Fix the bug.
3. Verify the test passes.
4. Update `CHANGELOG.md` under `[Unreleased]` → `Fixed`.
5. Run the full pipeline.

## Code conventions

- No `fmt.Println` or `os.Exit` in library packages (`pkg/`). Output goes through reporters; exits happen in `cmd/`.
- Exported interfaces live in `pkg/validator/validator.go` and `pkg/reporter/reporter.go`.
- Don't modify the `Validator`, `SchemaValidator`, or `Reporter` interfaces without discussion — these are public API.
- Validators are stateless structs. Options that change parse behavior use separate types (see `CsvValidator` with `Delimiter`, `Comment`, `LazyQuotes` fields).
- Use `var _ Validator = FooValidator{}` compile-time interface checks.
- Generated files end in `_gen.go`. Don't edit them manually; run `go generate`.
- Don't put test data inline in `_test.go` files when it's more than a few lines. Use `testdata/` directories or `internal/testhelper`.
- Don't add a new top-level package without discussing in an issue first. The package layout is intentional.
- Keep test coverage at or above 90%.
- Follow gofmt formatting. No exceptions.

## Dependency policy

- Prefer the standard library over third-party packages when the difference is marginal.
- No CGO. All dependencies must be pure Go. The project builds as a static binary with `CGO_ENABLED=0`.
- Justify new dependencies in the PR description. "It was convenient" is not sufficient.
- Pin exact versions in `go.mod` (Go modules do this by default).
- Check that a new dependency is actively maintained and has a compatible license (MIT, BSD, Apache 2.0).

## golangci-lint

The project uses a strict golangci-lint config (`.golangci.yaml`). Common issues that trip up contributors:

- **errorlint**: Use `errors.Is()` / `errors.As()` instead of `==` or type assertions on errors.
- **revive (exported)**: Every exported type, function, and method needs a comment. Comment must start with the identifier name.
- **nolintlint**: If you suppress a linter with `//nolint`, you must specify which linter and add an explanation: `//nolint:gosec // reason here`.
- **gosec**: Don't ignore. If it flags something, fix it or explain why it's safe.
- **mirror**: Use `bytes.Clone(b)` instead of `append([]byte(nil), b...)`, etc.
- **gci (formatter)**: Imports must be grouped: stdlib, third-party, then project imports (`github.com/Boeing/config-file-validator`). Separate each group with a blank line.

## Testing patterns

- Table-driven tests with descriptive names.
- `internal/testhelper` provides `CreateFixtureDir`, `CreateFixtureFile`, and `WriteFile`.
- Fuzz tests for validators that handle untrusted input (see `FuzzJsonValidator`, etc.).
- The `cmd/validator` package uses `txtar`-based testscript tests (`testscript_test.go`). Add new CLI integration tests as `.txtar` files in `cmd/validator/testdata/`. See `basic.txtar` for the pattern.

## Decisions and constraints

- Single static binary, zero runtime dependencies. No shelling out to external tools.
- Validators process untrusted input. Never shell out, never use `unsafe`, never trust file content. Use bounded reads for large files. The `gosec` linter catches common issues — don't suppress without explanation.
- The project uses `go-git/go-git/v5` for gitignore pattern matching (not the git CLI).
- Schema validation uses JSON Schema (via `xeipuuv/gojsonschema`) and XSD (via `lestrrat-go/helium`).
- SchemaStore integration fetches schemas from schemastore.org with local disk caching.
- `pkg/validator/justfile` is a regular package (not a separate module) containing a justfile lexer, parser, and semantic analyzer. It has no external dependencies.
- Don't edit `pkg/filetype/known_files_gen.go` — generated by `go generate`.
- Don't edit `coverage.out` or `*_cov.out` — test artifacts.

## Documentation site

The docs live in `website/` and use Docusaurus. To build and preview locally:

```
cd website
npm install
npm run build
npm run serve
```

For development with hot reload: `npm start` (from `website/`). Requires Node ≥ 20.

MegaLinter runs on PRs and checks YAML, JSON, and markdown formatting in addition to the Go pipeline. Fix any issues it flags.

OpenSSF Scorecard runs on PRs and checks supply-chain security. Common failures:

- GitHub Actions must be pinned to full commit SHAs, not tags (e.g. `uses: actions/checkout@de0fac2e...` not `actions/checkout@v4`).
- Workflows must declare minimal `permissions` (avoid `permissions: write-all`).
- Avoid dangerous workflow patterns like `pull_request_target` with explicit checkout of PR code.
