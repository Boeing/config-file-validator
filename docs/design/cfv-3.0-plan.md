# cfv 3.0 — Implementation Plan

## Current State — Resume Here

**Branch**: `feat/3.0`
**Last updated**: 2026-06-29
**Next task**: Phase 2, Task 5 — YAML formatter

### What's done

**Phase 1 ✅ Complete**
- Binary renamed `validator` → `cfv`. Module path v2→v3.
- Subcommand router: `cfv check`, `cfv format`, `cfv version`, `cfv help`
- `cfv check .` = identical to old `validator .`
- `cmd/validator/` deleted. No shim.
- All 18 website docs updated.
- Opus review done, 4 architectural issues fixed.

**Phase 2 — In progress**
- ✅ `Formatter` interface, `Options` struct, `IsFormatted` helper (`pkg/formatter/`)
- ✅ `Report` struct refactored to v3: `Status` enum (Pass/Fail/Unformatted), `Issues []Issue`. All 5 reporters updated.
- ✅ JSON formatter (`pkg/formatter/jsonfmt/`) — 11 fixture tests, idempotency, fuzz, all options
- ✅ `cfv format .` wired — reports unformatted files with `~` symbol, exit 1
- ✅ `cfv format --fix .` wired — atomic writes (temp + rename), exit 0
- ✅ `Formatter` field added to `FileType`. Registered in `pkg/filetype/formatters.go`.
- ✅ Parallel worker pool (`runtime.NumCPU()`) in `pkg/cli/format.go`

### What's next

**Task 5**: YAML formatter (`pkg/formatter/yamlfmt/`)
- Library: `gopkg.in/yaml.v3` Node API (already a dep, zero new deps)
- Key: decode into `*yaml.Node`, walk tree normalizing indent/style, encode back
- Comment preservation: native via `HeadComment`/`LineComment`/`FootComment` fields
- Fixtures: ≥10 (indent width, block style, quote style, comment preservation, multi-doc)
- After YAML: register in `formatters.go` switch, add `yaml` and `yml` cases

**Task 6** (after YAML): TOML, HCL, ENV, INI, XML, Properties formatters
- TOML: `pelletier/go-toml/v2` `unstable.Parser{KeepComments: true}`
- HCL: `hclwrite.Format(src)` — literally one call
- ENV: custom line-oriented formatter
- INI: `gopkg.in/ini.v1`
- XML: `go-xmlfmt/xmlfmt` (new dep, MIT, zero transitive)
- Properties: `magiconair/properties` (already a dep)

**Task 7**: `[format]` config section in `.cfv.toml`

**Task 8**: Stress test + Opus review

### How to start the next session

```
cd /Users/se456c/src/github.com/boeing/config-file-validator
git checkout feat/3.0
go test ./... # verify all green
```

Then say "let's keep going" — next task is the YAML formatter.

### Pipeline state

```
go vet ./...           ✅
gofmt -s -l -e .       ✅
golangci-lint run      ✅ 0 issues
go test ./...          ✅ all pass
coverage               ✅ 93.9%
```

---

## Vision

cfv is the universal config file toolkit. One binary that validates, formats, and fixes every config file in your repo.

**Binary name**: `cfv` (replaces `validator`)

**Tagline**: Validate. Format. Fix. Every config format.

---

## Guiding Principles

These are not aspirations. They are constraints. If an implementation decision conflicts with one of these, the decision changes, not the principle.

### 1. Good architecture beats speed

Ship slower. Do it right.

- A phase is not done when the code runs. It's done when the architecture is clean, the interfaces are right, and the tests prove it works.
- Resist the urge to cut corners to hit a timeline. A sloppy formatter that mostly works will haunt every future formatter that builds on the same infrastructure.
- If a design feels wrong while implementing it, stop and fix the design. Don't paper over it.
- The phase timeline estimates are rough. A phase taking 2x as long because the architecture got refactored mid-way is a success, not a failure.
- Code that can be extended without being rewritten is worth more than code that shipped fast.

### 2. Build for users, not for implementers

Every decision that touches the CLI, output, or config must be evaluated from the user's perspective first.

- Before adding a flag, ask: what does a user who has never seen this tool need to do, and is this the most obvious way to do it?
- Error messages are part of the product. "exit status 1" is not an error message. Tell the user what went wrong and what to do next.
- The fix suggestion in the summary line (`2 errors fixable with --fix`) is a first-class feature, not an afterthought. A user who runs `cfv .` for the first time should immediately know their next action.
- If a behavior surprises a reasonable user, that's a bug — even if it's technically correct.
- Defaults must be safe. `cfv .` never writes. `cfv --fix .` asks for nothing it doesn't need.
- The migration from v2 must be a one-line change in CI scripts (`validator .` → `cfv check .`). That's it.

---

## Process Discipline

Process is how principles become reality. Since this is one giant branch with a single PR at the end, process is about staying grounded and building incrementally.

### Throughout Development

1. **Keep the plan in sync**: Every significant decision, discovery, or pivot is logged in the plan immediately. Don't wait. The plan is your scratchpad AND your project journal.

2. **Document as you code**: 
   - Code comments explain *why*, not what
   - Commit messages log decisions and tradeoffs
   - Update specs as you discover constraints
   - Website docs are updated per-phase, not at the end

3. **Test before moving on**: No phase is "done" until tests prove it works. If tests fail, you fix them before moving to the next phase. No "we'll test it all at the end."

4. **Stress test every feature**:
   - Run against real-world configs (SchemaStore, popular repos, edge cases)
   - Fuzz for 5+ minutes — no crashes
   - Run CI pipeline — all checks pass
   - Before you consider it done, before you move to the next feature

5. **Deep code review with Claude Opus after each phase**:
   - Once all tests pass and stress tests pass for a phase, do a comprehensive Opus review
   - The reviewer reads the spec, the tests, the implementation
   - Address all feedback before proceeding to the next phase
   - If the review suggests significant changes, repeat the stress test after changes

### Review Checklist (Opus will use this for each phase)

**Spec Compliance**
- [ ] Implementation matches the spec exactly
- [ ] Behavior contracts are met (exit codes, error messages, output format)
- [ ] All edge cases documented in the spec are handled

**Architecture**
- [ ] Interfaces are clean and consistent
- [ ] Errors are descriptive and actionable
- [ ] No unexpected coupling between packages
- [ ] Comment preservation works (if applicable)
- [ ] Idempotency holds (if applicable)

**Testing**
- [ ] Fixture tests cover documented options
- [ ] Comment preservation tests exist (if applicable)
- [ ] Fuzz targets exist and run clean
- [ ] Integration tests prove end-to-end behavior
- [ ] Coverage ≥ target for the package
- [ ] Performance benchmarks show no regression

**User Experience**
- [ ] Error messages tell user what went wrong + what to do next
- [ ] Output format is consistent
- [ ] Defaults are sensible and safe
- [ ] Config options (if any) are discoverable
- [ ] Migration from v2 is seamless

**Future Proofing**
- [ ] Could a new format be added without breaking this?
- [ ] Could a new option be added without rewriting this?
- [ ] No unresolved TODOs (fix them, don't defer)
- [ ] Future maintainer would understand this in 6 months?

### Single PR at the End

When the entire feature is done (all phases complete, all tests passing, all Opus reviews addressed):

1. **Rebase to clean history** — organize commits by logical unit (CLI, first batch of formatters, fixer, reporter, etc.)
2. **Update CHANGELOG.md** — one entry per major component added
3. **Update website docs** — CLI reference, guides, examples, migration guide
4. **Final Opus review of the full PR** — spot check the entire integration
5. **Merge** — this is the v3.0.0 release commit

---

```shell
# The unified command — runs all checks (syntax, schema, formatting)
cfv .                              # report everything
cfv --fix .                        # fix everything safe
cfv --fix --unsafe .               # fix aggressively

# Granular subcommands
cfv check .                        # syntax + schema only
cfv check --fix .                  # fix safe syntax/schema issues
cfv check --fix --unsafe .         # aggressive syntax/schema fixes (type coercions)

cfv format .                       # report formatting issues only
cfv format --fix .                 # rewrite to canonical style
```

### Behavior Contracts

| Command | Reads files | Writes files | Exit 1 on issues |
|---------|------------|--------------|------------------|
| `cfv .` | ✅ | ❌ | ✅ |
| `cfv --fix .` | ✅ | ✅ (safe only) | ✅ (unfixable remain) |
| `cfv check .` | ✅ | ❌ | ✅ |
| `cfv check --fix .` | ✅ | ✅ | ✅ |
| `cfv format .` | ✅ | ❌ | ✅ |
| `cfv format --fix .` | ✅ | ✅ | ❌ (all fixable) |

### Output

```
$ cfv .

  × config.yml:5 — "8080" is string, schema expects integer
  × deploy/app.json:12 — trailing comma
  ~ main.toml — inconsistent indentation (tabs, expected 2 spaces)
  ~ .env — spaces around "=" (expected no spaces)
  ✓ 47 files passed

Found 4 issues (3 fixable with --fix, 1 with --unsafe)
```

### Backward Compatibility

- `cfv check .` is the exact equivalent of today's `validator .`
- `cfv .` is a superset (adds formatting checks)
- No `validator` binary ships — clean break, update your scripts
- `.cfv.toml` config file name unchanged

---

## Architecture

### Package Structure

```
cmd/cfv/                    CLI entrypoint, subcommand routing
pkg/validator/              Syntax validators (unchanged)
pkg/formatter/              NEW — Format engines per format
pkg/formatter/json/         JSON/JSONC formatting
pkg/formatter/yaml/         YAML formatting
pkg/formatter/toml/         TOML formatting
pkg/formatter/xml/          XML formatting
pkg/formatter/ini/          INI formatting
pkg/formatter/env/          ENV formatting
pkg/formatter/hcl/          HCL formatting
pkg/formatter/properties/   Properties formatting
pkg/formatter/hocon/        HOCON formatting
pkg/formatter/csv/          CSV formatting
pkg/formatter/kdl/          KDL formatting
pkg/formatter/cue/          CUE formatting
pkg/formatter/justfile/     Justfile formatting
pkg/formatter/plist/        PList formatting
pkg/fixer/                  NEW — Fix engines (syntax + schema)
pkg/filetype/               FileType registry (add Formatter field)
pkg/finder/                 Filesystem walker (unchanged)
pkg/reporter/               Output formatters (extended for format/fix results)
pkg/cli/                    CLI engine (extended for format/fix modes)
pkg/schemastore/            SchemaStore (unchanged)
pkg/configfile/             .cfv.toml parser (extended)
```

### Formatter Interface

```go
package formatter

// Formatter rewrites file content to canonical style.
type Formatter interface {
    // Format returns the canonically formatted version of src.
    // Returns src unchanged if already formatted.
    Format(src []byte, opts Options) ([]byte, error)

    // IsFormatted reports whether src matches canonical style.
    IsFormatted(src []byte, opts Options) (bool, []Diff, error)
}

// Options are per-format configuration. Each format uses what applies.
type Options struct {
    IndentWidth      int    // spaces per level (0 = tabs)
    UseTabs          bool
    MaxLineWidth     int    // 0 = unlimited
    TrailingNewline  bool
    SortKeys         bool
    LineEnding       string // "lf", "crlf", "auto"
    QuoteStyle       string // "double", "single", "preserve"
    TrailingComma    string // "always", "never", "preserve" (JSONC, TOML)
    SpaceAroundEquals bool  // INI, ENV, Properties
    InsertFinalNewline bool
}

// Diff represents a single formatting difference.
type Diff struct {
    Line    int
    Message string
}
```

### Fixer Interface

```go
package fixer

// Fix represents a single correctable issue.
type Fix struct {
    Line     int
    Column   int
    Message  string
    Category FixCategory // Syntax, Schema, Format
    Safety   FixSafety   // Safe, Unsafe
}

// Fixer produces fixes for a given file.
type Fixer interface {
    // Fixes analyzes src and returns available fixes.
    Fixes(src []byte, schema *Schema) []Fix

    // Apply applies the given fixes to src and returns corrected content.
    Apply(src []byte, fixes []Fix) ([]byte, error)
}

type FixCategory int

const (
    FixSyntax FixCategory = iota
    FixSchema
    FixFormat
)

type FixSafety int

const (
    Safe   FixSafety = iota
    Unsafe
)
```

### FileType Extension

```go
// FileType gains a Formatter field.
type FileType struct {
    Name       string
    Extensions map[string]struct{}
    Validator  validator.Validator
    Formatter  formatter.Formatter  // NEW — nil if not yet implemented
    Fixer     fixer.Fixer           // NEW — nil if not yet implemented
}
```

---

## Format Specifications Per Format

### JSON
- **Library**: `encoding/json` + `tidwall/pretty`
- **Options**: indent width, tabs, sort keys, trailing newline, max line width
- **Comment preservation**: N/A (no comments in JSON)
- **Defaults**: 2 spaces, sorted keys, trailing newline, no trailing comma

### JSONC
- **Library**: `tidwall/jsonc` + `tidwall/pretty`
- **Options**: same as JSON + trailing comma control
- **Comment preservation**: preserve comments in-place (format around them)
- **Defaults**: 2 spaces, trailing newline, trailing commas allowed

### YAML
- **Library**: `gopkg.in/yaml.v3` Node API (already a dep, zero new deps)
- **Options**: indent width, quote style (single/double/preserve), flow vs block, max line width, document start marker, indentless arrays
- **Comment preservation**: ✅ via Node.HeadComment/LineComment/FootComment fields (native round-trip)
- **Defaults**: 2 spaces, block style, double quotes, no document start marker

### TOML
- **Library**: `pelletier/go-toml/v2` `unstable.Parser` with `KeepComments: true`
- **Options**: indent width, align entries, trailing comma in arrays, array expand/collapse, reorder keys
- **Comment preservation**: ✅ — parse with `KeepComments`, comments become `Node{Kind: Comment}` with exact byte ranges. Format structural nodes, splice comments back at their relative positions.
- **Defaults**: no indent (TOML convention), align entries off, trailing newline
- **Strategy**: Use go-toml's `unstable.Parser` (already a dep) to get a full AST with comments as first-class nodes. Record each comment's attachment point (which expression it precedes/follows/is inline with). Reformat structural content (spacing, alignment, blank lines). Re-insert comments at their original relative positions. No custom lexer needed.

### XML
- **Library**: `go-xmlfmt/xmlfmt` (MIT, zero deps, regex-based, preserves comments)
- **Options**: indent width, tabs, self-closing tags, attribute quote style, attribute sorting
- **Comment preservation**: ✅
- **Defaults**: 2 spaces, double-quote attributes, no attribute sorting

### INI
- **Library**: `gopkg.in/ini.v1` (Apache 2.0, zero deps)
- **Options**: space around `=`, blank lines between sections, section ordering (alpha/preserve), key ordering within sections
- **Comment preservation**: ✅
- **Defaults**: spaces around `=`, blank line between sections, preserve ordering

### ENV
- **Library**: Custom (line-oriented, trivial to build)
- **Options**: space around `=`, key ordering (alpha/preserve), key casing enforcement (UPPERCASE), blank lines, quoting style
- **Comment preservation**: ✅ (line-oriented, comments are just lines starting with #)
- **Defaults**: no spaces around `=`, UPPERCASE keys, no blank lines between entries, quote values containing spaces

### Properties
- **Library**: `magiconair/properties` (BSD, already a dep)
- **Options**: separator style (`=`, `:`, space), key ordering, space around separator, encoding (UTF-8 vs ISO-8859-1)
- **Comment preservation**: ✅ (line-oriented)
- **Defaults**: `=` separator, spaces around `=`, preserve ordering

### HCL
- **Library**: `hashicorp/hcl/v2` hclwrite (MPL 2.0, already a dep)
- **Options**: None (canonical formatting, like `terraform fmt`)
- **Comment preservation**: ✅
- **Defaults**: 2-space indent, aligned `=`, canonical HashiCorp style
- **Note**: `hclwrite.Format(src)` is literally one function call. Done.

### HOCON
- **Library**: Custom (no formatter exists anywhere)
- **Options**: indent width, brace style, include resolution
- **Comment preservation**: ✅ (line-oriented approach)
- **Defaults**: 2 spaces, opening brace on same line
- **Strategy**: Line-oriented formatter. Normalize indent and spacing without full re-serialization (HOCON is too complex for a full AST round-trip in v3.0).

### CSV
- **Library**: `encoding/csv` (stdlib)
- **Options**: delimiter, quoting style (minimal/always/never), trim whitespace, trailing newline, header normalization
- **Comment preservation**: N/A
- **Defaults**: comma delimiter, minimal quoting, trailing newline

### KDL
- **Library**: `sblinch/kdl-go` (check for printer) or custom
- **Options**: indent width
- **Comment preservation**: TBD
- **Defaults**: 4 spaces (KDL convention)

### CUE
- **Library**: `cuelang.org/go/cue/format` (Apache 2.0, already a dep)
- **Options**: indent width, simplify (remove redundant syntax)
- **Comment preservation**: ✅ (cue/format preserves comments)
- **Defaults**: tabs (CUE convention, matching `cue fmt`)

### Justfile
- **Library**: Custom (your parser, your formatter)
- **Options**: indent width (recipe bodies), blank lines between recipes
- **Comment preservation**: ✅ (line-oriented)
- **Defaults**: 4 spaces for recipe bodies, 1 blank line between recipes

### PList (Apple XML)
- **Library**: `howett.net/plist` (already a dep) + `go-xmlfmt/xmlfmt`
- **Options**: indent width (XML mode)
- **Comment preservation**: ✅ (XML comments preserved by xmlfmt)
- **Defaults**: tabs (Apple Xcode convention)

### TOON
- **Library**: Custom
- **Options**: Same as TOML (TOON is TOML-based)
- **Comment preservation**: Same strategy as TOML
- **Defaults**: Same as TOML

### SARIF
- **Library**: `tidwall/pretty` (it's just JSON)
- **Options**: Same as JSON
- **Comment preservation**: N/A (JSON)
- **Defaults**: 2 spaces, sorted keys, trailing newline

---

## Fix Specifications

### Safe Fixes (--fix)

| Category | Fix | Formats |
|----------|-----|---------|
| Syntax | Remove trailing comma | JSON |
| Syntax | Add missing trailing newline | All |
| Syntax | Remove BOM | All |
| Syntax | Normalize line endings | All |
| Syntax | Remove trailing whitespace | All |
| Syntax | Fix dangling comma in arrays | JSON, TOML |
| Schema | `"8080"` → `8080` (string→integer) | JSON, YAML, TOML |
| Schema | `"true"` → `true` (string→boolean) | JSON, YAML, TOML |
| Schema | `"3.14"` → `3.14` (string→number) | JSON, YAML, TOML |
| Schema | Case-mismatch enum: `"True"` → `"true"` | All with schema |
| Format | Normalize indentation | All |
| Format | Normalize spacing around delimiters | INI, ENV, Properties |
| Format | Sort keys (when configured) | JSON, YAML, TOML, INI, ENV, Properties |
| Format | Normalize quote style | YAML, XML |
| Format | Add/remove trailing commas (JSONC) | JSONC |

### Unsafe Fixes (--fix --unsafe)

| Category | Fix | Formats | Risk |
|----------|-----|---------|------|
| Schema | `8080` → `"8080"` (integer→string) | JSON, YAML, TOML | Might break consumers expecting int |
| Schema | Unwrap single-element array | JSON, YAML | `[x]` → `x` per schema |
| Syntax | Remove duplicate keys (keep last) | JSON, YAML, TOML | Might remove intended override |
| Format | Convert flow→block style | YAML | Changes readability |
| Format | Collapse multiline→single line | JSON, YAML | Changes readability |

---

## Configuration (.cfv.toml)

```toml
# Existing keys (unchanged)
search-paths = ["."]
exclude-dirs = ["node_modules", "vendor", ".git"]
reporter = ["standard"]
gitignore = true

# NEW: formatting configuration
[format]
indent = 2
use-tabs = false
max-line-width = 120
trailing-newline = true
sort-keys = false
line-ending = "lf"

# Per-format overrides
[format.json]
sort-keys = true
indent = 2

[format.yaml]
quote-style = "double"
indent = 2

[format.toml]
align-entries = true

[format.ini]
space-around-equals = true

[format.env]
space-around-equals = false
key-casing = "upper"

[format.hcl]
# No options — canonical style

[format.cue]
use-tabs = true  # CUE convention

# NEW: fix configuration
[fix]
unsafe = false                    # default safe-only
exclude-rules = ["sort-keys"]    # skip specific fixes
```

---

## Migration Path (v2 → v3)

### Breaking Changes

1. **Binary name**: `validator` → `cfv` (no compatibility shim — update your scripts)
2. **Default behavior**: `cfv .` reports formatting issues in addition to syntax/schema (more output than before)
3. **Module path**: `github.com/Boeing/config-file-validator/v3`
4. **Minimum Go version**: 1.22+ (for range-over-int, slices package)

### Migration Guide

| v2 | v3 |
|----|-----|
| `validator .` | `cfv check .` (exact equivalent) |
| `validator --reporter=json .` | `cfv check --reporter=json .` |
| `validator --fix` (did not exist) | `cfv --fix .` |

---

## Implementation Phases

Every phase follows Process Discipline: update plan before/during/after, write specs, stress test, Opus review. No phase is done until all three of these are true:

1. **Spec and plan are updated** — the plan reflects what was built, and the spec is accurate
2. **Tests prove it works** — fixture tests, integration tests, fuzz tests, stress tests all pass
3. **Deep review is complete** — Opus has reviewed the architecture, all feedback addressed

### Phase 1: Foundation ✅ COMPLETE

**Goal**: Ship `cfv check .` with identical behavior to `validator .`

1. ✅ Create `cmd/cfv/` entrypoint with subcommand routing
2. ✅ Wire `cfv check` to existing validation pipeline
3. ✅ `cfv .` (bare) delegates to `cfv check` initially
4. ✅ Add `--fix` and `--unsafe` flags (no-op initially, reserved)
5. ✅ Update module path to v3
6. ✅ Remove `cmd/validator/` — no compat shim, clean break
7. ✅ Update Homebrew formula, GitHub Action, pre-commit hook (deferred to Phase 5)

**Outcome**: `cfv check .` is functionally identical to the old `validator .`. All tests pass. Coverage 93.9%. Zero lint issues.

### Phase 2: Formatting Engine (4-6 weeks)

**Goal**: Ship `cfv format .` and `cfv format --fix .`

**Hard constraint**: Output formatting must be visually identical across all formatters. Every formatter reports issues using the same line format, symbols, and structure as `cfv check`. It must look like one tool, not a patchwork of libraries glued together. Define the output contract once, enforce it in every formatter's reporter integration. If a formatter can't produce a consistent message shape, fix the formatter — don't let it output garbage.

1. ✅ Define `Formatter` interface
2. ✅ Define the output contract for formatting issues (~ symbol, same reporter pipeline as check)
3. Implement formatters in priority order:
   a. ✅ JSON (`tidwall/pretty`) — done
   b. 🔲 YAML (`gopkg.in/yaml.v3` Node API) — **NEXT**
   c. 🔲 TOML (`pelletier/go-toml/v2` unstable.Parser with KeepComments)
   d. 🔲 HCL (`hclwrite.Format`) — one function call
   e. 🔲 ENV (custom, line-oriented)
   f. 🔲 INI (`gopkg.in/ini.v1`)
   g. 🔲 XML (`go-xmlfmt/xmlfmt`)
   h. 🔲 Properties (`magiconair/properties`)
4. ✅ Register formatters on FileType (`pkg/filetype/formatters.go`)
5. ✅ `cfv format .` reports unformatted files with ~ symbol (does not write)
6. ✅ `cfv format --fix .` rewrites files atomically (temp + rename)
7. `cfv .` (bare command) stays as check-only until all formatters are stable
8. Add `[format]` section to `.cfv.toml` parser
9. ✅ Reporters updated via Report v3 refactor (StatusUnformatted, IssueTypeFormat)
10. ✅ Exit codes correct (1 on unformatted, 0 all pass)

### Phase 3: Formatters Continued (2-3 weeks)

**Goal**: Complete all format coverage

1. CUE (`cuelang.org/go/cue/format`)
2. HOCON (custom line-oriented)
3. KDL (custom or via library)
4. Justfile (custom, your parser)
5. PList (xmlfmt on the XML output)
6. CSV (custom, trivial)
7. TOON (same as TOML)
8. SARIF (same as JSON)
9. JSONC (tidwall/jsonc + tidwall/pretty, comment handling)

### Phase 4: Fix Engine (3-4 weeks)

**Goal**: Ship `cfv --fix .` and `cfv check --fix .`

1. Define `Fixer` interface
2. Implement safe syntax fixes:
   - Trailing comma removal (JSON)
   - Trailing newline insertion (all)
   - BOM removal (all)
   - Line ending normalization (all)
   - Trailing whitespace removal (all)
3. Implement safe schema fixes:
   - String→integer coercion
   - String→boolean coercion
   - String→number coercion
   - Enum case normalization
4. Wire fix engine into `cfv check --fix`
5. Add `--unsafe` flag with unsafe fixes
6. Output messaging: "N fixable with --fix, M with --unsafe"
7. `cfv --fix .` applies both format fixes and check fixes

### Phase 5: Polish & Ship (2-3 weeks)

**Goal**: Production-ready v3.0.0 release

1. Documentation site update (new CLI reference, format guide, migration guide)
2. README rewrite (new name, new capabilities, new demo)
3. Benchmark regression suite wired into CI
4. Fuzz test corpus seeded from real-world config files (schemastore, popular repos)
5. GitHub Action update
6. Pre-commit hook update
7. Homebrew formula update (new binary name)
8. Release v3.0.0-rc1 for community testing
9. Release v3.0.0

---

## New Dependencies

| Package | License | Purpose | Deps |
|---------|---------|---------|------|
| `tidwall/pretty` | MIT | JSON formatting | Zero |
| `tidwall/jsonc` | MIT | JSONC comment stripping | Zero |
| `go-xmlfmt/xmlfmt` | MIT | XML formatting | Zero |
| `gopkg.in/ini.v1` | Apache 2.0 | INI formatting | Zero |

Total: 4 new packages, all MIT/Apache, all zero transitive deps.

**Already in go.mod** (no new deps needed):
- `gopkg.in/yaml.v3` — YAML formatting (Node API)
- `pelletier/go-toml/v2` — TOML formatting (unstable.Parser with KeepComments)
- `magiconair/properties` — Properties formatting
- `hashicorp/hcl/v2` — HCL formatting (hclwrite)
- `cuelang.org/go` — CUE formatting (cue/format)

---

## Timeline

| Phase | Duration | Milestone |
|-------|----------|-----------|
| Phase 1: Foundation | 2-3 weeks | `cfv check .` works, binary renamed |
| Phase 2: Core Formatters | 4-6 weeks | JSON, YAML, TOML, HCL, ENV, INI, XML, Properties |
| Phase 3: Remaining Formatters | 2-3 weeks | All 18 formats covered |
| Phase 4: Fix Engine | 3-4 weeks | `--fix` and `--unsafe` work |
| Phase 5: Polish & Ship | 2-3 weeks | v3.0.0 release |
| **Total** | **13-19 weeks** | |

---

## Testing Strategy

Every feature ships with tests that prove it works. Tests are part of the definition of done for each phase — not a Phase 5 afterthought.

### Formatter Tests

Each formatter gets four categories of tests:

**1. Fixture round-trip tests** (`pkg/formatter/<format>/testdata/`)

For each format, a directory of `.input` / `.expected` file pairs:
```
pkg/formatter/json/testdata/
  indent_2_spaces.input.json
  indent_2_spaces.expected.json
  indent_tabs.input.json
  indent_tabs.expected.json
  sort_keys.input.json
  sort_keys.expected.json
  already_formatted.input.json
  already_formatted.expected.json
  trailing_newline_missing.input.json
  trailing_newline_missing.expected.json
```

The test loops over all pairs:
```go
func TestFormat(t *testing.T) {
    inputs, _ := filepath.Glob("testdata/*.input.*")
    for _, input := range inputs {
        expected := strings.Replace(input, ".input.", ".expected.", 1)
        // read both, run Format(), assert bytes.Equal(result, expectedBytes)
    }
}
```

Minimum fixture count per format: **10** (covering each option combination and edge case).

**2. Idempotency tests**

For every `.expected` file, assert `Format(expected) == expected`. If formatting the already-formatted output produces different output, the formatter has a bug.

```go
func TestIdempotency(t *testing.T) {
    for _, expected := range expectedFiles {
        result, _ := formatter.Format(expected, opts)
        if !bytes.Equal(result, expected) {
            t.Errorf("not idempotent: re-formatting %s produces different output", file)
        }
    }
}
```

**3. Comment preservation tests**

For every format that supports comments, a dedicated fixture containing:
- Inline comments
- Block comments (above a key)
- Trailing comments (end of section)
- Comments inside arrays/objects (where applicable)
- Comments at the start and end of file

The test asserts every comment string from the input appears in the output:

```go
func TestCommentPreservation(t *testing.T) {
    comments := extractComments(input)  // regex or format-specific extraction
    result, _ := formatter.Format(input, opts)
    for _, comment := range comments {
        if !bytes.Contains(result, []byte(comment)) {
            t.Errorf("comment lost: %q", comment)
        }
    }
}
```

**4. Fuzz tests**

For each formatter, a fuzz target that feeds random valid inputs and asserts:
- No panic
- If Format returns nil error, the output is valid syntax (re-parse succeeds)
- Idempotency: `Format(Format(x)) == Format(x)`

```go
func FuzzJSONFormatter(f *testing.F) {
    f.Add([]byte(`{"key": "value"}`))
    f.Fuzz(func(t *testing.T, data []byte) {
        result, err := jsonFormatter.Format(data, defaultOpts)
        if err != nil {
            return // unparseable input, skip
        }
        if !json.Valid(result) {
            t.Fatal("formatter produced invalid JSON")
        }
        result2, _ := jsonFormatter.Format(result, defaultOpts)
        if !bytes.Equal(result, result2) {
            t.Fatal("not idempotent")
        }
    })
}
```

### Fixer Tests

**1. Per-rule fixture tests** (`pkg/fixer/testdata/`)

Each fix rule gets `.input` / `.expected` / `.fixes.json` triplets:
```
pkg/fixer/testdata/
  json_trailing_comma.input.json
  json_trailing_comma.expected.json
  json_trailing_comma.fixes.json     ← expected Fix structs (rule ID, line, safety)
  yaml_string_to_int.input.yaml
  yaml_string_to_int.expected.yaml
  yaml_string_to_int.fixes.json
```

Tests assert:
- `Fixes()` returns the expected fix list (correct rule IDs, positions, safety levels)
- `Apply()` produces the expected output
- The fixed output passes validation (syntax + schema)

**2. Overlap tests**

Dedicated tests with inputs that produce overlapping fixes. Assert:
- The leftmost fix wins
- Dropped fixes are reported (not silently lost)
- The output is still valid

**3. Safety gate tests**

- Apply with `Safe` only → unsafe fixes are NOT applied
- Apply with `Safe + Unsafe` → all fixes applied
- Assert `exclude-rules` correctly suppresses specific rule IDs

**4. Roundtrip: fix → format → validate**

End-to-end test that takes a broken file, runs the full pipeline (fixer → formatter → validator), and asserts the final output is valid and formatted:
```go
func TestFixFormatValidate(t *testing.T) {
    // Input: invalid JSON with trailing comma + bad indent
    // After fixer: trailing comma removed
    // After formatter: indent normalized
    // Assert: valid JSON, formatted, passes schema
}
```

### CLI Integration Tests (testscript/txtar)

The existing `cmd/validator/testscript_test.go` pattern continues for `cmd/cfv/`. Each behavior contract from the plan gets a txtar test:

```txtar
# cfv check reports syntax errors and exits 1
! exec cfv check .
stdout '×'

-- bad.json --
{"key": "value",}
```

```txtar
# cfv format reports formatting issues without writing
exec cfv format .
stdout '~'
cmp messy.json messy.json.orig

-- messy.json --
{"key":   "value"}
-- messy.json.orig --
{"key":   "value"}
```

```txtar
# cfv format --fix rewrites files and then passes
exec cfv format --fix .
exec cfv format .
! stdout '~'

-- messy.json --
{"key":   "value"}
```

```txtar
# cfv --fix applies safe fixes, reports remaining unsafe
! exec cfv --fix .
stdout 'fixable with --fix --unsafe'

-- .cfv.toml --
[schema-map]
"bad.json" = "schema.json"

-- bad.json --
{"port": "8080"}
-- schema.json --
{"type": "object", "properties": {"port": {"type": "integer"}}}
```

Minimum txtar tests: **15 per subcommand** covering:
- Exit codes for each combination (issues found, no issues, tool error)
- `--fix` writes files, `--fix --unsafe` writes more
- `--reporter json` produces valid JSON output
- `--reporter sarif` produces valid SARIF output
- `--quiet` suppresses stdout output
- stdin mode (`-` argument) works
- `.cfv.toml` options are respected
- CLI flag takes precedence over config file
- `--no-config` ignores config file
- `--gitignore` skips ignored files
- Multiple search paths work
- `--exclude-dirs` and `--exclude-file-types` filter correctly
- Unknown subcommand produces helpful error
- `--version` prints version and exits 0
- `--help` prints help and exits 0

### Reporter Tests

**1. Snapshot tests** (golden files) for each reporter:

```go
func TestJSONReporter(t *testing.T) {
    reports := fixtureReports() // fixed set of issues: syntax, schema, format, with/without fixes
    var buf bytes.Buffer
    reporter := NewJSONReporter("")
    reporter.PrintTo(reports, &buf)
    golden := readFile("testdata/json_output.golden")
    if !bytes.Equal(buf.Bytes(), golden) {
        t.Errorf("output differs from golden file; run with -update to refresh")
    }
}
```

Update golden files with `-update` flag. Reviewed in PR diffs.

**2. Schema validation** of structured output:

- JSON reporter output validated against a JSON Schema (checked into repo)
- SARIF reporter output validated against the SARIF 2.1.0 schema
- JUnit reporter output validated against the JUnit XSD

This catches structural regressions where the output looks right but violates the spec consumers expect.

**3. Fix metadata in reports:**

- JSON reporter includes `fix` field on fixable issues, absent on non-fixable
- SARIF reporter includes `fixes` array with `artifactChanges`
- Summary line shows correct fixable/unfixable counts
- After `--fix`, fixed issues are absent from output

### Performance Benchmarks

Benchmarks for hot paths, run in CI. If a benchmark regresses >20% vs main, the PR is flagged.

```go
func BenchmarkJSONFormat_1KB(b *testing.B) {
    src := readFile("testdata/small.json") // ~1KB
    opts := defaultOpts
    for i := 0; i < b.N; i++ {
        jsonFormatter.Format(src, opts)
    }
}

func BenchmarkJSONFormat_100KB(b *testing.B) {
    src := readFile("testdata/large.json") // ~100KB
    opts := defaultOpts
    for i := 0; i < b.N; i++ {
        jsonFormatter.Format(src, opts)
    }
}

func BenchmarkFixerApply(b *testing.B) { ... }
func BenchmarkDiffCompute(b *testing.B) { ... }
func BenchmarkFinderWalk_10K_Files(b *testing.B) { ... }
```

Targets:
- <1ms per file for typical configs (<10KB)
- <10ms per file for large configs (<100KB)
- <100ms for finder walk on 10K-file tree

### Coverage Requirements

| Package | Minimum |
|---------|---------|
| Overall | ≥ 90% |
| `pkg/formatter/*` | ≥ 95% (pure functions, no excuse) |
| `pkg/fixer/` | ≥ 95% |
| `pkg/reporter/` | ≥ 90% |
| `pkg/cli/` | ≥ 85% (orchestration, some paths hard to unit test) |
| `cmd/cfv/` | Covered by txtar integration tests |

### CI Pipeline (extended from AGENTS.md)

```
go vet ./...
test -z "$(gofmt -s -l -e .)"
golangci-lint run ./...
go generate ./pkg/filetype/...
go build -o /dev/null cmd/cfv/cfv.go
go test -cover -coverprofile coverage.out ./...
go tool cover -func coverage.out | grep total
# Fuzz tests: 30s per formatter in CI
go test -fuzz=FuzzJSONFormatter -fuzztime=30s ./pkg/formatter/json/
go test -fuzz=FuzzYAMLFormatter -fuzztime=30s ./pkg/formatter/yaml/
go test -fuzz=FuzzTOMLFormatter -fuzztime=30s ./pkg/formatter/toml/
# ... one per format that has a fuzz target
# Benchmarks: compare to main, flag regressions
go test -bench=. -benchmem -count=5 ./pkg/formatter/... ./pkg/fixer/... | tee bench.txt
benchstat main.bench.txt bench.txt
```

### When a Test Is Required

| Change | Required tests |
|--------|---------------|
| New formatter | Fixture round-trips (≥10), idempotency, comment preservation, fuzz target |
| New fix rule | Per-rule fixture, overlap test if edits could collide, safety gate |
| New CLI flag | txtar test asserting the flag works and interacts correctly with config |
| New reporter format | Golden file snapshot, schema validation of output |
| Bug fix | Regression test that fails without the fix, passes with it |
| Performance change | Benchmark showing improvement (or no regression) |

---

## Success Metrics

- Every format has a formatter (18/18) with ≥10 fixture tests each
- `cfv format --fix .` is idempotent (proven by idempotency tests on every fixture)
- Formatting preserves all comments (proven by comment preservation tests per format)
- Fuzz tests run 30s per format in CI with zero crashes
- Formatting is fast (<1ms for <10KB, <10ms for <100KB — proven by benchmarks)
- Zero new CGO deps (remains static binary)
- Coverage ≥ 90% overall, ≥ 95% for formatter and fixer packages
- All existing v2 tests pass under `cfv check`
- Every fix rule has a fixture test proving correct behavior
- Every CLI behavior contract has a txtar integration test
- JSON/SARIF reporter output validates against its respective schema
- The README demo makes someone say "I need this"

---

## Decisions

1. **EditorConfig**: No. `.cfv.toml` is the single source of truth.
2. **Parallel formatting**: Yes. Worker pool at `runtime.NumCPU()`.
3. **Fix loop**: Single-pass (biome-style), not multi-pass (eslint-style).
4. **Format issues severity**: Warning, not error. Files with only format issues are "valid."
5. **Arg parsing library**: Stay with `flag`. Thin subcommand router, no new deps.
6. **Default behavior of `cfv format .`**: Report-only (no write). Matches biome.
7. **Unknown config keys**: Error. A config validator must not silently accept bad config.
8. **Formatter interface**: Single `Format(src []byte, opts Options) ([]byte, error)` method. `IsFormatted` = byte equality comparison. No separate method.
9. **Fixer position model**: Byte-range text edits (like eslint/ruff), not AST reconstruction.
10. **Schema fixes get byte positions**: SchemaErrors enhanced to carry byte offsets so the fixer can locate values precisely.
11. **Comment preservation**: Non-negotiable. Every formatter MUST preserve comments. For TOML: use `unstable.Parser{KeepComments: true}` from `pelletier/go-toml/v2` (already a dep). For YAML: use `gopkg.in/yaml.v3` Node API (already a dep, comments preserved via HeadComment/LineComment/FootComment fields). No format will ever drop user comments.
12. **Binary name**: `cfv`. No conflicts on Homebrew.
13. **YAML formatter library**: `gopkg.in/yaml.v3` Node API. Zero new deps.

---

## Open Questions

None. All design questions resolved.
