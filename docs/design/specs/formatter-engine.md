# Formatter Engine Spec — cfv 3.0

Status: Draft
Date: 2026-06-29

## Overview

cfv 3.0 adds a formatting engine that can check whether config files match a canonical style and optionally rewrite them. The engine lives in `pkg/formatter/` and integrates with the existing `pkg/cli` orchestration loop.

Two CLI modes use the engine:

- `cfv format .` — format-only mode. Runs formatters, does not validate syntax or schema.
- `cfv .` — combined mode. Validates first, then checks formatting on files that passed validation.

---

## Formatter Interface

```go
package formatter

// Formatter transforms source bytes into canonically formatted output.
// Implementations MUST be stateless and safe for concurrent use.
// Implementations MUST preserve all comments in the source.
type Formatter interface {
    Format(src []byte, opts Options) ([]byte, error)
}
```

One method. No `IsFormatted` — that's derived by byte equality:

```go
formatted, err := f.Format(src, opts)
isFormatted := err == nil && bytes.Equal(src, formatted)
```

If you need a diff, compare `src` to `formatted` using the standalone diff package.

### Comment Preservation (non-negotiable)

Every formatter MUST preserve all comments in the source file. This is a hard constraint, not a best-effort goal.

- If a format supports comments (YAML, TOML, HCL, INI, XML, ENV, Properties, HOCON, JSONC, KDL, CUE, Justfile, PList), the formatter output MUST contain every comment from the input.
- If a library's round-trip (decode→encode) drops comments, do NOT use that library for formatting. Build a token-level or line-oriented formatter instead.
- Comments may be repositioned (e.g., moved to align with reformatted code) but never deleted.
- This constraint is tested: for every format with comment support, there's a test fixture containing comments, and the test asserts all comments survive formatting.

---

## Options

```go
package formatter

// Options controls formatting behavior. Zero value means "use format-specific defaults."
type Options struct {
    IndentStyle  IndentStyle // Tabs or Spaces
    IndentWidth  int         // Spaces per indent level. Ignored when IndentStyle is Tabs.
    MaxLineWidth int         // 0 = no limit
    FinalNewline bool        // Ensure file ends with a single newline
    LineEnding   LineEnding  // LF, CRLF, or Auto (detect from file)
    SortKeys     bool        // Sort object/map keys alphabetically
}

// IndentStyle selects between spaces and tabs.
type IndentStyle int

const (
    IndentSpaces IndentStyle = iota
    IndentTabs
)

// LineEnding selects the line terminator.
type LineEnding int

const (
    LineEndingLF   LineEnding = iota // \n
    LineEndingCRLF                   // \r\n
    LineEndingAuto                   // Detect from file content; default to LF if ambiguous
)
```

### Option Resolution

Options resolve top-down. Highest priority wins; unset fields fall through to the next level.

```
CLI flags                          ← highest priority
.cfv.toml [format.<filetype>]      ← per-format override
.cfv.toml [format]                 ← global format settings
Format-specific hardcoded defaults ← lowest priority (set by each Formatter impl)
```

No EditorConfig support. `.cfv.toml` is the single source of truth for formatting options.

#### Example

Given `.cfv.toml`:

```toml
[format]
indent-style = "spaces"
indent-width = 2
final-newline = true

[format.yaml]
indent-width = 4
```

And CLI invocation:

```
cfv format --indent-width 8 .
```

Resolution for a `.yaml` file:

| Field        | CLI  | format.yaml | format | Default | Resolved |
|--------------|------|-------------|--------|---------|----------|
| IndentStyle  | —    | —           | spaces | spaces  | spaces   |
| IndentWidth  | 8    | 4           | 2      | 2       | **8**    |
| FinalNewline | —    | —           | true   | false   | true     |

CLI flag `--indent-width 8` wins over all config layers.

Resolution for a `.json` file (no `[format.json]` section):

| Field        | CLI  | format.json | format | Default | Resolved |
|--------------|------|-------------|--------|---------|----------|
| IndentStyle  | —    | —           | spaces | spaces  | spaces   |
| IndentWidth  | 8    | —           | 2      | 2       | **8**    |
| FinalNewline | —    | —           | true   | false   | true     |

---

## Diff Computation

Diff is a standalone package, not part of the Formatter interface.

```go
package diff

// Hunk represents a contiguous block of changes.
type Hunk struct {
    OrigStart int      // 1-based line number in original
    OrigCount int      // Number of lines from original
    NewStart  int      // 1-based line number in formatted
    NewCount  int      // Number of lines in formatted
    Lines     []Line   // Context, additions, and removals
}

// Line is a single line in a diff hunk.
type Line struct {
    Kind    LineKind
    Content string // Without trailing newline
}

// LineKind classifies a line within a hunk.
type LineKind int

const (
    KindContext  LineKind = iota // Unchanged line (prefix: " ")
    KindRemoval                 // Line in original but not formatted (prefix: "-")
    KindAddition                // Line in formatted but not original (prefix: "+")
)

// Compute produces a minimal set of hunks describing the differences between
// original and formatted. Context lines default to 3 (matching git diff).
// Returns nil if the inputs are identical.
func Compute(original, formatted []byte) []Hunk
```

Line-based comparison. Lines are split on `\n` (with `\r\n` normalized to `\n` before splitting). This matches what `git diff`, `prettier --check`, `rustfmt --check`, and `dprint check` produce.

The diff package has no dependency on the formatter package.

---

## File Write Algorithm

Used by `cfv format --fix .` to rewrite files in place.

```
function WriteFormatted(path string, formatted []byte) error:
    // 1. Read original permissions
    info = os.Stat(path)
    perm = info.Mode().Perm()

    // 2. Create temp file in same directory (same filesystem)
    dir = filepath.Dir(path)
    tmp = os.CreateTemp(dir, ".cfv-fmt-*")
    defer func():
        if tmp still exists:
            os.Remove(tmp.Name())  // Never leave temp files behind

    // 3. Write formatted content
    tmp.Write(formatted)
    tmp.Close()

    // 4. Set permissions to match original
    os.Chmod(tmp.Name(), perm)

    // 5. Atomic rename
    err = os.Rename(tmp.Name(), path)
    if err != nil:
        // Windows cross-device fallback: write in place
        os.WriteFile(path, formatted, perm)

    // 6. tmp no longer exists (renamed), defer cleanup is a no-op
    return nil
```

Properties:
- Atomic on POSIX (rename is a single syscall).
- Original file is never in a half-written state.
- Temp file is always cleaned up, even on panic (deferred removal).
- Permissions preserved exactly.

---

## Error Handling Matrix

| Failure Mode | Behavior | Exit Code | Reporter Output |
|---|---|---|---|
| File can't be read (permissions, missing) | Skip file, continue | 1 | Error row for that file |
| Formatter returns error (unparseable input) | Skip file, continue | 1 | Error row: "parse error: ..." |
| Temp file creation fails (disk full, no write perms to dir) | Skip file, continue | 1 | Error row: "write failed: ..." |
| Rename fails and in-place write also fails | Skip file, continue | 1 | Error row: "write failed: ..." |
| File is already formatted | No action | 0 | Reported as passing (or omitted in quiet mode) |
| File needs formatting (check mode, no --fix) | Report diff | 1 | Diff hunks shown |
| File needs formatting (--fix mode) | Rewrite file | 0 | Reported as fixed |
| FileType has no Formatter (nil) | Silently skip | — | Not included in output |

Key principle: never abort the batch. Process all files, collect all results, report at the end.

---

## Concurrency Model

```
                    ┌─────────────┐
                    │  Finder     │
                    │  (walks FS) │
                    └──────┬──────┘
                           │ []FileMetadata
                           ▼
                    ┌─────────────┐
                    │  Dispatcher │
                    └──────┬──────┘
                           │ sends files to worker pool
              ┌────────────┼────────────┐
              ▼            ▼            ▼
        ┌──────────┐ ┌──────────┐ ┌──────────┐
        │ Worker 1 │ │ Worker 2 │ │ Worker N │   N = runtime.NumCPU()
        │ Format() │ │ Format() │ │ Format() │
        └────┬─────┘ └────┬─────┘ └────┬─────┘
             │             │             │
             └─────────────┼─────────────┘
                           │ results via channel
                           ▼
                    ┌─────────────┐
                    │  Collector  │
                    │  (sorts by  │
                    │   path)     │
                    └──────┬──────┘
                           │ sorted []FormatReport
                           ▼
                    ┌─────────────┐
                    │  Reporter   │
                    └─────────────┘
```

- Worker pool size: `runtime.NumCPU()`.
- Each file is independent. Formatters hold no mutable state.
- Results collected into a slice, sorted by `FilePath` before reporting.
- Deterministic output regardless of goroutine scheduling.
- File reads and writes happen inside workers (I/O parallelism matters for large repos on SSDs).

```go
type FormatResult struct {
    FilePath  string
    Original  []byte
    Formatted []byte
    Err       error    // Non-nil means the file was skipped
}
```

---

## Registering Formatters on FileType

`pkg/filetype/file_type.go` gains a `Formatter` field:

```go
import "github.com/Boeing/config-file-validator/v3/pkg/formatter"

type FileType struct {
    Name       string
    Extensions []string
    KnownFiles []string
    Validator  validator.Validator
    Formatter  formatter.Formatter // nil = no formatter for this type
}
```

Registration follows the same pattern as validators:

```go
var JsonFileType = FileType{
    Name:       "json",
    Extensions: []string{".json", ".geojson"},
    Validator:  validator.JsonValidator{},
    Formatter:  formatter.JsonFormatter{},
}
```

A nil `Formatter` is valid. `cfv format .` silently skips file types where `ft.Formatter == nil`.

---

## Idempotency Contract

Every formatter implementation MUST satisfy:

```
Format(Format(src, opts), opts) == Format(src, opts)
```

For all valid inputs and all option combinations.

### Testing

Each formatter has a round-trip fuzz test:

```go
func FuzzJsonFormatterIdempotent(f *testing.F) {
    f.Add([]byte(`{"a":1}`))
    f.Fuzz(func(t *testing.T, src []byte) {
        opts := formatter.Options{IndentStyle: formatter.IndentSpaces, IndentWidth: 2}
        first, err := formatter.JsonFormatter{}.Format(src, opts)
        if err != nil {
            t.Skip() // Unparseable input — not a formatter bug
        }
        second, err := formatter.JsonFormatter{}.Format(first, opts)
        if err != nil {
            t.Fatalf("formatted output is not parseable: %v", err)
        }
        if !bytes.Equal(first, second) {
            t.Fatalf("idempotency violation:\nfirst:  %q\nsecond: %q", first, second)
        }
    })
}
```

CI runs fuzz tests with `-fuzztime=30s` per formatter. A violation is a release blocker.

Additionally, a deterministic table test runs every fixture file through `Format` twice and asserts byte equality:

```go
func TestAllFormattersIdempotent(t *testing.T) {
    for _, ft := range filetype.FileTypes {
        if ft.Formatter == nil {
            continue
        }
        // Walk testdata/ for files matching ft.Extensions
        // Format twice, assert equality
    }
}
```

---

## Integration with the CLI Loop

### Format-Only Mode (`cfv format .`)

```go
func RunFormat(finder Finder, opts formatter.Options, fix bool, reporter Reporter) int {
    files := finder.Find()

    results := make(chan FormatResult, len(files))
    pool := newWorkerPool(runtime.NumCPU())

    for _, file := range files {
        pool.Submit(func() {
            ft := file.FileType
            if ft.Formatter == nil {
                return // silently skip
            }

            content, err := os.ReadFile(file.Path)
            if err != nil {
                results <- FormatResult{FilePath: file.Path, Err: err}
                return
            }

            formatted, err := ft.Formatter.Format(content, opts)
            if err != nil {
                results <- FormatResult{FilePath: file.Path, Err: err}
                return
            }

            if fix && !bytes.Equal(content, formatted) {
                if err := writeFormatted(file.Path, formatted); err != nil {
                    results <- FormatResult{FilePath: file.Path, Err: err}
                    return
                }
            }

            results <- FormatResult{
                FilePath:  file.Path,
                Original:  content,
                Formatted: formatted,
            }
        })
    }

    pool.Wait()
    close(results)

    // Collect, sort by path, report
    sorted := collectAndSort(results)
    return reporter.PrintFormatResults(sorted)
}
```

### Combined Mode (`cfv .`)

In `cli.Run()`, formatting runs as a second pass after validation:

```go
func (c *CLI) Run() int {
    files := c.Finder.Find()

    // Pass 1: Validate syntax and schema (existing logic)
    validationReports := c.validateAll(files)

    // Pass 2: Check formatting (only on files that passed validation)
    var formatReports []FormatResult
    if c.FormatOpts != nil {
        passed := filterPassed(files, validationReports)
        formatReports = c.checkFormatting(passed)
    }

    // Merge results and report
    return c.Reporter.Print(validationReports, formatReports)
}
```

In combined mode, formatting never runs with `--fix`. It only checks. The user runs `cfv format --fix .` explicitly to rewrite files.

---

## Package Layout

```
pkg/formatter/
├── formatter.go       // Formatter interface, Options, IndentStyle, LineEnding
├── json.go            // JsonFormatter
├── yaml.go            // YamlFormatter
├── toml.go            // TomlFormatter
├── hcl.go             // HclFormatter
├── xml.go             // XmlFormatter
├── options.go         // Option resolution logic (merge layers)
├── options_test.go
└── formatter_test.go  // Idempotency tests, shared test infrastructure

pkg/diff/
├── diff.go            // Compute function, Hunk, Line types
└── diff_test.go
```

---

## CLI Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--fix` | bool | false | Rewrite files in place (only valid with `cfv format`) |
| `--indent-style` | string | — | `spaces` or `tabs` |
| `--indent-width` | int | — | Spaces per indent level |
| `--line-ending` | string | — | `lf`, `crlf`, or `auto` |
| `--final-newline` | bool | — | Ensure trailing newline |
| `--sort-keys` | bool | — | Sort object keys |

Flags without a value mean "unset" — fall through to config file or defaults. The zero value of `Options` fields means "use next resolution layer," not "use this value."

---

## .cfv.toml Format Section

```toml
[format]
indent-style = "spaces"
indent-width = 2
final-newline = true
line-ending = "lf"

[format.json]
indent-width = 4
sort-keys = true

[format.yaml]
indent-width = 2

[format.hcl]
# HCL uses canonical style; options are ignored.
# This section exists for documentation, not configuration.
```

---

## Reporter Extension

`reporter.Report` gains optional format fields:

```go
type Report struct {
    // Existing fields
    FileName        string
    FilePath        string
    IsValid         bool
    ValidationError string

    // New: formatting results
    IsFormatted   *bool    // nil = not checked, true = matches, false = differs
    FormatDiff    []byte   // Unified diff (only populated when IsFormatted == false)
    FormatError   string   // Non-empty when formatter returned an error
    WasFixed      bool     // true if --fix rewrote this file
}
```

`IsFormatted` is a pointer so reporters can distinguish "not checked" from "checked and passed." Reporters that don't understand formatting (e.g., an older JUnit consumer) ignore nil fields.

---

## Exit Codes

| Scenario | Exit Code |
|----------|-----------|
| All files valid and formatted | 0 |
| Some files need formatting (check mode) | 1 |
| Some files had errors (parse, write) | 1 |
| All files fixed successfully (--fix mode) | 0 |
| Mix of fixed files and errors (--fix mode) | 1 |
| No config files found | 0 |

---

## Constraints

- No external binaries. Formatters are pure Go, in-process.
- No CGO. Consistent with the project's static binary policy.
- Formatters must not allocate unbounded memory. For files larger than 10 MB, formatters may return an error rather than OOM.
- Formatters must not modify semantics. Formatting changes whitespace, key order (if `SortKeys`), and trailing newlines. It never changes values, adds keys, or removes content.
