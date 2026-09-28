# AGENTS.md

Context for AI coding agents working on config-file-validator.

## Project overview

Config File Validator (`cfv`) is a Go CLI tool that validates syntax, enforces schemas, and checks/fixes formatting across 18 configuration file formats. Single static binary, zero runtime dependencies.

- Module: `github.com/Boeing/config-file-validator/v3`
- Go: 1.26+
- Entry: `cmd/cfv/cfv.go`
- License: Apache 2.0

## Architecture

```
cmd/cfv/                    Subcommand router, flag parsing, config resolution, watch mode
pkg/cli/                    Orchestrator: wires finder → validators → formatters → fixers → reporters
pkg/validator/              Validator implementations (one file per format, 18 total)
pkg/validator/justfile/     Justfile lexer, parser, AST, analyzer (pure Go, no external deps)
pkg/filetype/               FileType registry: name → extensions → validator → formatter
pkg/finder/                 Filesystem walker with gitignore, exclude, depth, globbing
pkg/formatter/              Formatter interface, Options, config readers (prettier, taplo, yamlfmt, editorconfig)
pkg/formatter/jsonfmt/      JSON formatter (delegates to jsoncfmt with trailing commas removed)
pkg/formatter/jsoncfmt/     JSONC formatter (CST-based via hujson, comment-preserving)
pkg/formatter/yamlfmt/      YAML formatter (custom tokenizer + printer)
pkg/formatter/tomlfmt/      TOML formatter (custom tokenizer + grouper + printer)
pkg/formatter/xmlfmt/       XML formatter (custom tokenizer + printer, mixed content preservation)
pkg/formatter/hclfmt/       HCL formatter (delegates to hcl/v2/hclwrite)
pkg/formatter/inifmt/       INI formatter (custom lexer + parser + printer)
pkg/formatter/envfmt/       ENV formatter (custom line-oriented)
pkg/formatter/propfmt/      Properties formatter (custom tokenizer + printer)
pkg/fixer/                  Fix engine: trailing commas, schema-driven type coercion
pkg/reporter/               Output formatters: stdout, JSON, JUnit, SARIF (with merge), GitHub Actions
pkg/schemastore/            SchemaStore catalog lookup and caching
pkg/configfile/             .cfv.toml config file parser with embedded JSON Schema
pkg/tools/                  Small utilities (arrToMap, glob, file URL)
internal/generate/          Code generator: known filenames from GitHub Linguist
internal/testhelper/        Test helpers: fixture creation, valid/invalid content maps
```

## Subcommands

| Command | Description |
|---------|-------------|
| `cfv check [flags] [paths]` | Validate syntax → schema → format check. Default when no subcommand given. |
| `cfv format [flags] [paths]` | Format check only. `--fix` rewrites files, `--diff` shows unified diff. |
| `cfv version` | Print version and exit. |
| `cfv help [subcommand]` | Print help. |

## Data flow

### check

```
parseCheckFlags → resolveCheckConfig (CLI > env > .cfv.toml > defaults)
  → buildFormatOptionsResolver (Tier 1 or Tier 2)
  → buildCLI → cli.Run()
    → finder.Find() → []FileMetadata
    → per file:
        ReadFile → strip BOM
        → ValidateSyntax
        → if valid: validateSchema (inline → --schema-map → --schemastore)
        → if --fix and errors: attemptFix (trailing commas, type coercion)
        → if no errors and formatter exists: checkFormatting (optionally --fix)
        → build Report (Pass / Fail / Unformatted)
    → printReports via reporters
```

### format

```
parseFormatFlags → resolveFormatConfig (no schema resolution)
  → buildFormatOptionsResolver
  → buildCLI → cli.Format(optsResolver)
    → finder.Find() → filter to files with Formatter
    → skip format-ignored files
    → parallel worker pool (NumCPU):
        ReadFile → Format(content, opts)
        → if same: Pass
        → if different + --fix: writeFileAtomic → Pass
        → if different + --diff: unified diff → Unformatted
        → if different: Unformatted
    → printReports
```

### Config resolution priority

CLI flag > environment variable > `.cfv.toml` > hardcoded default

### Format options resolution

**Tier 1** — `.cfv.toml` exists: sole authority. External tool configs NOT read.

```
formatDefaults → [format] section → [format.<type>] section → CLI flags
```

**Tier 2** — no `.cfv.toml`: per-format tool ownership.

```
formatDefaults → .editorconfig → ONE tool config → CLI flags
```

| Format | Tool config |
|--------|-------------|
| JSON, JSONC | `.prettierrc` |
| YAML | `.yamlfmt` if found, else `.prettierrc` |
| TOML | `taplo.toml` |
| HCL, XML, INI, Properties, ENV | `.editorconfig` only |

## cmd/cfv/ file breakdown

| File | Purpose |
|------|---------|
| `cfv.go` | Subcommand router. Routes to `runCheck()`, `runFormat()`, `version`, `help`. |
| `flags.go` | Flag registration for check and format subcommands. Validation helpers. |
| `builder.go` | Constructs `cli.CLI` from resolved config. Builds reporters, finder, schema store. |
| `config.go` | Config merging: CLI flags + env vars + `.cfv.toml`. Defines `cfvConfig` and `resolvedConfig`. |
| `format_opts.go` | Two-tier format options resolution. Loads `.prettierrc`, `taplo.toml`, `.yamlfmt`, `.editorconfig`. |
| `watch.go` | `--watch` mode via fsnotify. Debounced re-validation on file changes. |

## Interfaces

### Validator (`pkg/validator/validator.go`)

```go
type Validator interface {
    ValidateSyntax(b []byte) (bool, error)
}

type SchemaValidator interface {
    ValidateSchema(b []byte, filePath string) (bool, error)
}

type JSONMarshaler interface {
    MarshalToJSON(b []byte) ([]byte, error)
}

type XMLSchemaValidator interface {
    ValidateXSD(b []byte, schemaPath string) (bool, error)
}
```

All validators are stateless structs with compile-time checks: `var _ Validator = FooValidator{}`.

### Formatter (`pkg/formatter/formatter.go`)

```go
type Formatter interface {
    Format(src []byte, opts Options) ([]byte, error)
}
```

Stateless, concurrent-safe, idempotent, comment-preserving. Returns `ErrSkipped` when a file can't be processed but isn't a syntax error.

### Reporter (`pkg/reporter/reporter.go`)

```go
type Reporter interface {
    Print(reports []Report) error
}
```

Five implementations: `StdoutReporter`, `JSONReporter`, `JunitReporter`, `SARIFReporter`, `GitHubReporter`.

### FileType (`pkg/filetype/file_type.go`)

```go
type FileType struct {
    Name       string
    Extensions map[string]struct{}
    KnownFiles map[string]struct{}
    Validator  validator.Validator
    Formatter  formatter.Formatter  // nil = no formatter
}
```

18 types registered. Formatters wired in `wireFormatters()` called from `init()`. 9 formats have formatters: JSON, JSONC, YAML, TOML, XML, HCL, INI, ENV, Properties. The rest have `nil` Formatter.

## Local pipeline

Run all of these in order before pushing. Every check must pass.

```
go vet ./...
test -z "$(gofmt -s -l -e .)"
golangci-lint run ./...
go generate ./pkg/filetype/...
go build -o /dev/null ./cmd/cfv/
go test -cover -coverprofile coverage.out ./...
go tool cover -func coverage.out | grep total
```

Coverage must be ≥ 90%.

For fast iteration on a single package:

```
go test ./pkg/validator/...
go test ./pkg/formatter/yamlfmt/...
go test ./pkg/cli/...
```

## Quick reference

```
go test -v -run TestFoo ./pkg/validator/...                          # Run one test
go test -count=1 ./pkg/validator/...                                 # Skip test cache
go build -o ./cfv ./cmd/cfv/ && ./cfv check .                       # Build and run
go build -o ./cfv ./cmd/cfv/ && ./cfv format --diff .               # Check formatting
go test -bench=. -benchmem ./pkg/finder/...                          # Benchmark finder
go test -fuzz FuzzYAMLFormatter -fuzztime 30s ./pkg/formatter/yamlfmt/...  # Fuzz a formatter
```

## Prerequisites

- Go 1.26+ (see `go.mod`)
- golangci-lint v2 (`go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`)
- Node.js ≥ 20 (only for documentation site in `website/`)

## Adding a new validator

1. Create `pkg/validator/<format>.go`:

```go
package validator

type FooValidator struct{}

var _ Validator = FooValidator{}

func (FooValidator) ValidateSyntax(b []byte) (bool, error) {
    // Parse b. Return (true, nil) on success or (false, err) on failure.
    // Wrap errors in &ValidationError{Err: ..., Line: ...} when position is available.
}
```

2. Optionally implement `SchemaValidator`, `JSONMarshaler`, or `XMLSchemaValidator` if the format supports schema validation.

3. Register in `pkg/filetype/file_type.go`:
   - Add a package-level `var FooFileType = FileType{...}` with name, extensions, and validator.
   - Add the name → pointer entry to `fileTypeRegistry`.
   - Add the value to the `FileTypes` slice in `init()`.

4. If the format supports formatting, add a formatter (see next section).

5. Add test cases in `pkg/validator/validator_test.go`. Follow the existing table-driven style. Add a fuzz target.

6. Run `go generate ./pkg/filetype/...` if the format has known filenames in GitHub Linguist.

7. Update documentation:
   - `website/docs/reference/supported-file-types.md`
   - `website/docs/guides/file-type-detection.md` (if the format has known filenames)
   - `CHANGELOG.md` under `[Unreleased]` → `Added`

## Adding a new formatter

1. Create `pkg/formatter/<fmtname>/`:
   - `<format>.go` — Implement `formatter.Formatter` interface with a `Format(src []byte, opts Options) ([]byte, error)` method.
   - `<format>_test.go` — Golden file tests using `testdata/*.input.<ext>` / `testdata/*.expected.<ext>` pairs. Use `fixture_opts.go` for per-fixture option overrides via `.opts.json` sidecar files.
   - `<format>_rules_test.go` — Unit tests for individual formatting rules.
   - Add fuzz targets: at minimum `FuzzFormat` and `Fuzz<Name>Formatter`.

2. Wire it in `pkg/filetype/formatters.go` — add a case to `wireFormatters()`.

3. If the format has an external tool config (like `.prettierrc` for JSON), add a config reader in `pkg/formatter/` and integrate it into `cmd/cfv/format_opts.go`.

4. Update documentation:
   - `website/docs/guides/formatting.md`
   - `website/docs/reference/supported-file-types.md`
   - `CHANGELOG.md` under `[Unreleased]` → `Added`

## Adding a new reporter

1. Create `pkg/reporter/<name>_reporter.go` implementing `Reporter`:

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

2. Wire it into `cmd/cfv/builder.go` in `getReporter()`.

3. Add tests in `pkg/reporter/reporter_test.go`.

4. Update documentation:
   - `website/docs/guides/output-reporters.md`
   - `website/docs/reference/cli-flags.md`
   - `CHANGELOG.md` under `[Unreleased]` → `Added`

## Fixing a bug

1. Write a test that reproduces the bug (it should fail before your fix).
2. Fix the bug.
3. Verify the test passes.
4. Update `CHANGELOG.md` under `[Unreleased]` → `Fixed`.
5. Run the full pipeline.

## PR requirements

1. Update `CHANGELOG.md` under `[Unreleased]`. Follow [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) format.
2. Pass all CI checks (vet, fmt, build, test, coverage ≥ 90%).
3. Pass `golangci-lint run ./...` with zero findings.
4. Conventional commit messages: `type(scope): description`. Types: `feat`, `fix`, `docs`, `chore`, `ci`, `test`, `refactor`.

## Code conventions

- No `fmt.Println` or `os.Exit` in library packages (`pkg/`). Output goes through reporters; exits happen in `cmd/`.
- Exported interfaces live in `pkg/validator/validator.go`, `pkg/formatter/formatter.go`, and `pkg/reporter/reporter.go`. Don't modify without discussion — these are public API.
- Validators and formatters are stateless structs. Options that change behavior use struct fields (see `CsvValidator`).
- Use `var _ Validator = FooValidator{}` compile-time interface checks.
- Generated files end in `_gen.go`. Don't edit manually; run `go generate`.
- Test data goes in `testdata/` directories, not inline in `_test.go` files.
- Don't add a new top-level package without discussing in an issue first.
- Keep test coverage ≥ 90%.
- Follow gofmt formatting.

## Dependency policy

- Prefer the standard library when the difference is marginal.
- No CGO. All dependencies must be pure Go. The project builds with `CGO_ENABLED=0`.
- Justify new dependencies in the PR description.
- Pin exact versions in `go.mod`.
- Check that a new dependency is actively maintained and has a compatible license (MIT, BSD, Apache 2.0).

## golangci-lint

The project uses a strict config (`.golangci.yaml`) with `revive` in enable-all-rules mode. Common issues:

- **errorlint**: Use `errors.Is()` / `errors.As()` instead of `==` or type assertions on errors.
- **revive (exported)**: Every exported type, function, and method needs a doc comment starting with the identifier name.
- **nolintlint**: `//nolint` requires a specific linter and explanation: `//nolint:gosec // reason here`.
- **gosec**: Don't suppress without explanation.
- **mirror**: Use `bytes.Clone(b)` instead of `append([]byte(nil), b...)`.
- **gci**: Imports grouped: stdlib, third-party, project (`github.com/Boeing/config-file-validator`). Blank line between groups.

## Testing patterns

### Unit tests
Table-driven with descriptive names. Uses `testify/require` for assertions. `internal/testhelper` provides `CreateFixtureDir`, `CreateFixtureFile`, `WriteFile`, and `ValidContent`/`InvalidContent` maps per file type.

### Formatter golden file tests
Each formatter sub-package uses `testdata/*.input.<ext>` → `testdata/*.expected.<ext>` pairs. Per-fixture option overrides via `testdata/*.opts.json` sidecar files, loaded by `pkg/formatter/fixture_opts.go`. Run with `-update` to regenerate golden files.

### txtar integration tests
`cmd/cfv/testdata/*.txtar` files (79 total) test CLI behavior end-to-end via `rogpeppe/go-internal/testscript`. The `cfv` binary runs in-process. Add new CLI tests as `.txtar` files. See `basic.txtar` for the pattern.

### Fuzz tests
~45 fuzz targets across validators, formatters, and the fixer. Every validator and every formatter has at least one. Run with:

```
go test -fuzz FuzzFoo -fuzztime 30s ./pkg/formatter/yamlfmt/...
```

### Stress tests
- `cmd/cfv/stress_format_test.go` — Semantic equivalence, idempotency, and real-world corpus tests across all formatted formats.
- `pkg/validator/justfile/stress_test.go` — 2000 randomly generated justfiles + 40 adversarial patterns.
- `cmd/cfv/testdata/stress_*.txtar` — Integration-level stress tests.

## Decisions and constraints

- Single static binary, zero runtime dependencies. No shelling out to external tools.
- Validators process untrusted input. Never shell out, never use `unsafe`, never trust file content.
- `go-git/go-git/v5` for gitignore pattern matching (not the git CLI).
- JSON Schema validation uses `santhosh-tekuri/jsonschema/v6`. XSD validation uses `lestrrat-go/helium`.
- SchemaStore integration fetches schemas from schemastore.org with local disk caching under `~/.cache/cfv/schemas/`.
- `pkg/validator/justfile/` is a regular package (not a separate module) containing a full justfile lexer, parser, and semantic analyzer with no external dependencies.
- Don't edit `pkg/filetype/known_files_gen.go` — generated by `go generate` from GitHub Linguist data.

## Documentation site

Docs live in `website/` and use Docusaurus.

```
cd website && npm install && npm run build && npm run serve
```

For development with hot reload: `npm start` (from `website/`). Requires Node ≥ 20.

## CI workflows

| Workflow | Trigger | Purpose |
|----------|---------|---------|
| `go.yml` | Push to main, PRs | Vet, fmt, generated file check, build, test (coverage ≥ 90%) |
| `golangci-lint.yml` | Push to main, PRs | golangci-lint on 3 OS matrix |
| `mega-linter.yml` | Push, PRs to main | MegaLinter: YAML, JSON, markdown, shell, security scanners |
| `release.yml` | Release created | Cross-platform binaries, SHA256SUMS, SLSA provenance, AUR publish |
| `changelog.yml` | PRs to main | Verifies CHANGELOG.md was updated |
| `linguist.yml` | Weekly (Monday), manual | Fetches latest Linguist data, regenerates known files, opens PR |
| `docs.yml` | Push to main (website/**) | Builds and deploys Docusaurus site to GitHub Pages |
| `scorecard.yml` | Weekly, push to main | OpenSSF Scorecard supply-chain security |

OpenSSF Scorecard requirements: GitHub Actions pinned to full commit SHAs (not tags), minimal workflow permissions, no `pull_request_target` with PR checkout.
