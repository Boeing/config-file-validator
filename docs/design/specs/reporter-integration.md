# Reporter Integration Spec — cfv 3.0

Module path: `github.com/Boeing/config-file-validator/v3`

This spec defines the data model, reporter output formats, and behavioral contracts for cfv 3.0's reporting subsystem.

## Data Model

### Core Types

```go
package reporter

// Report represents the validation results for a single file.
type Report struct {
    FilePath string
    FileName string
    Issues   []Issue
    Notes    []string // informational, not issues (e.g., "valid JSONC")
}

// IsValid returns true when the file has no errors.
// A file with only warnings (format issues, schema deprecations) is valid.
func (r Report) IsValid() bool {
    for _, issue := range r.Issues {
        if issue.Severity == SeverityError {
            return false
        }
    }
    return true
}

// Issue represents a single diagnostic found during validation.
type Issue struct {
    Type      IssueType // Syntax, Schema, Format
    Severity  Severity  // Error, Warning, Note
    Message   string    // human-readable description
    RuleID    string    // machine-readable identifier (e.g., "json/trailing-comma")
    Line      int       // 1-based, 0 if unknown
    Column    int       // 1-based, 0 if unknown
    EndLine   int       // 1-based, 0 if unknown
    EndColumn int       // 1-based, 0 if unknown
    Fix       *FixInfo  // nil if not fixable
}

// FixInfo describes how to automatically resolve an issue.
type FixInfo struct {
    Safety  Safety // Safe or Unsafe
    Message string // human-readable fix description (e.g., "remove trailing comma")
    Edits   []Edit // ordered text replacements to apply
}

// Edit is a single text replacement within a file.
type Edit struct {
    StartLine   int    // 1-based
    StartColumn int    // 1-based
    EndLine     int    // 1-based
    EndColumn   int    // 1-based (exclusive)
    NewText     string // replacement text (empty string = deletion)
}

// IssueType classifies the source of an issue.
type IssueType int

const (
    IssueTypeSyntax IssueType = iota // parse failure
    IssueTypeSchema                  // schema violation
    IssueTypeFormat                  // style/formatting
)

// Severity indicates the impact of an issue.
type Severity int

const (
    SeverityError   Severity = iota // file is invalid
    SeverityWarning                 // file is valid but has problems
    SeverityNote                    // informational
)

// Safety classifies whether a fix can be applied without human review.
type Safety int

const (
    SafetySafe   Safety = iota // fix preserves semantics
    SafetyUnsafe               // fix may change semantics
)
```

### String Representations

```go
func (t IssueType) String() string {
    switch t {
    case IssueTypeSyntax:
        return "syntax"
    case IssueTypeSchema:
        return "schema"
    case IssueTypeFormat:
        return "format"
    default:
        return "unknown"
    }
}

func (s Severity) String() string {
    switch s {
    case SeverityError:
        return "error"
    case SeverityWarning:
        return "warning"
    case SeverityNote:
        return "note"
    default:
        return "unknown"
    }
}

func (s Safety) String() string {
    switch s {
    case SafetySafe:
        return "safe"
    case SafetyUnsafe:
        return "unsafe"
    default:
        return "unknown"
    }
}
```

### Issue Type Mapping

| Condition | IssueType | Severity |
|-----------|-----------|----------|
| Parse failure (invalid syntax) | Syntax | Error |
| Schema violation (wrong type, missing required field) | Schema | Error |
| Schema deprecation (deprecated key used) | Schema | Warning |
| Format issue (wrong indentation, spacing) | Format | Warning |

Notes (e.g., "this file is valid JSONC") live on `Report.Notes`, not as issues.

## Reporter Interface

```go
// Reporter formats and outputs validation results.
type Reporter interface {
    Print(reports []Report) error
}
```

The interface signature is unchanged from v2. The breaking change is the `Report` struct it receives. This is acceptable because v3 uses a new module path.

## IsValid Derivation

`Report.IsValid()` is a method, not a stored field. A file is valid if and only if it has zero issues with `Severity == SeverityError`.

Consequences:
- A file with only format warnings is valid.
- A file with only schema warnings (deprecated keys) is valid.
- A file with one syntax error and ten format warnings is invalid.

## Exit Code Determination

Exit codes depend on the subcommand:

| Subcommand | Exit 0 | Exit 1 |
|------------|--------|--------|
| `cfv check .` | No errors (warnings OK) | One or more errors |
| `cfv format .` | No format warnings | One or more format warnings |
| `cfv .` (default) | No errors AND no format warnings | Any error or format warning |

When `--fix` is used and resolves all issues that would trigger exit 1, exit code is 0.

## Standard (Stdout) Reporter

### Output Format

Each issue prints on one line:

```
  × <path>:<line>:<col> — <message> [<rule_id>]
  ~ <path> — <message> [<rule_id>]
```

Symbols:
- `×` — error (Severity == Error)
- `~` — warning (Severity == Warning)

Position is included only when Line > 0. Column is included only when both Line > 0 and Column > 0.

```
  × config.yml:5:12 — "8080" is string, schema expects integer [schema/string-to-int]
  × deploy/app.json:12:5 — trailing comma [json/trailing-comma]
  ~ main.toml — inconsistent indentation (tabs, expected 2 spaces) [format/indent]
  ~ .env — spaces around "=" (expected no spaces) [format/spacing]
```

After all issues, a pass summary (only when at least one file has no issues):

```
  ✓ 47 files passed
```

Final summary line:

```
Found 4 issues (2 errors, 2 warnings)
  2 errors fixable with --fix
  2 warnings fixable with cfv format --fix
```

### Summary Line Logic

The summary line is constructed from counts:

| Condition | Summary |
|-----------|---------|
| errors > 0, warnings > 0 | `Found N issues (E errors, W warnings)` |
| errors > 0, warnings == 0 | `Found E errors` |
| errors == 0, warnings > 0 | `All checks passed. W formatting issues` |
| errors == 0, warnings == 0 | (no summary line printed) |

Fix hints appear as indented lines below the summary, only when fixable issues exist:

| Condition | Hint |
|-----------|------|
| fixable errors (safe) > 0 | `E errors fixable with --fix` |
| fixable errors (unsafe) > 0 | `E errors fixable with --fix --unsafe` |
| fixable warnings > 0 | `W warnings fixable with cfv format --fix` |

Safe and unsafe fixable errors get separate lines. If all fixable errors are safe, only one line appears.

### --quiet Behavior

Suppress all per-file lines. Print only the final summary line (or nothing if no issues).

### --verbose Behavior (reserved, not in v3.0)

Show the source line with a pointer under the error position. Not implemented — the flag is reserved so no reporter needs to handle it yet.

### GroupBy Behavior

GroupBy groups output by a field before printing. Supported values:

| GroupBy | Behavior |
|---------|----------|
| `directory` | Group files by parent directory. Print directory header before its files. |
| `type` | Group by IssueType. Print "Syntax errors:", "Schema errors:", "Format issues:" headers. |
| `severity` | Group by Severity. Print "Errors:", "Warnings:" headers. |
| (none) | Print issues in filesystem walk order. |

Within each group, issues are printed in the order they appear in the `[]Report` slice (filesystem walk order), then by line number within a file.

## JSON Reporter

### Output Shape

```json
{
  "summary": {
    "total_files": 51,
    "files_with_issues": 4,
    "errors": 2,
    "warnings": 2,
    "fixable_errors": 2,
    "fixable_warnings": 2
  },
  "results": [
    {
      "file_path": "config.yml",
      "file_name": "config.yml",
      "is_valid": false,
      "notes": [],
      "issues": [
        {
          "type": "schema",
          "severity": "error",
          "message": "\"8080\" is string, schema expects integer",
          "rule_id": "schema/string-to-int",
          "line": 5,
          "column": 12,
          "end_line": 5,
          "end_column": 18,
          "fix": {
            "safety": "safe",
            "message": "Convert string to integer",
            "edits": [
              {
                "start_line": 5,
                "start_column": 12,
                "end_line": 5,
                "end_column": 18,
                "new_text": "8080"
              }
            ]
          }
        }
      ]
    }
  ]
}
```

### Field Rules

- `is_valid`: derived from `Report.IsValid()`.
- `notes`: always present (empty array if none).
- `issues`: always present (empty array if none).
- `fix`: omitted (`null` in Go's default JSON marshaling) when `Issue.Fix` is nil.
- `end_line`, `end_column`: included as `0` when unknown. Consumers check for 0 to detect "unknown."
- Files with no issues are included in `results` (with empty `issues` array). This differs from v2 behavior where passing files were omitted. Rationale: tooling needs the full file list for diff-based reporting.

### --quiet Behavior

JSON reporter ignores `--quiet`. Full structure is always emitted. Rationale: JSON output is consumed by machines, not humans.

### GroupBy Behavior

When GroupBy is set, `results` is replaced by `groups`:

```json
{
  "summary": { ... },
  "group_by": "directory",
  "groups": [
    {
      "name": "deploy/",
      "results": [ ... ]
    }
  ]
}
```

`results` and `groups` are mutually exclusive top-level keys.

## SARIF Reporter

### Mapping

| cfv concept | SARIF element |
|-------------|---------------|
| Tool name + version | `tool.driver.name`, `tool.driver.version` |
| RuleID | `tool.driver.rules[].id` |
| IssueType | `tool.driver.rules[].properties.issueType` |
| Issue | `results[]` |
| Severity=Error | `level: "error"` |
| Severity=Warning | `level: "warning"` |
| Severity=Note | `level: "note"` |
| Line/Column | `locations[].physicalLocation.region` |
| FixInfo | `results[].fixes[]` |
| Safety | `results[].fixes[].properties.safety` |
| Edit | `fixes[].artifactChanges[].replacements[]` |

### Fix Representation

```json
{
  "fixes": [
    {
      "description": {
        "text": "Convert string to integer"
      },
      "artifactChanges": [
        {
          "artifactLocation": {
            "uri": "config.yml"
          },
          "replacements": [
            {
              "deletedRegion": {
                "startLine": 5,
                "startColumn": 12,
                "endLine": 5,
                "endColumn": 18
              },
              "insertedContent": {
                "text": "8080"
              }
            }
          ]
        }
      ],
      "properties": {
        "safety": "safe"
      }
    }
  ]
}
```

### Rules Array

All distinct `RuleID` values across all reports are collected into `tool.driver.rules[]`:

```json
{
  "tool": {
    "driver": {
      "name": "config-file-validator",
      "version": "3.0.0",
      "rules": [
        {
          "id": "json/trailing-comma",
          "shortDescription": { "text": "Trailing comma in JSON" },
          "properties": {
            "issueType": "syntax"
          }
        },
        {
          "id": "schema/string-to-int",
          "shortDescription": { "text": "String value where integer expected" },
          "properties": {
            "issueType": "schema"
          }
        }
      ]
    }
  }
}
```

Each result references its rule by index: `"ruleIndex": 0`.

### --quiet Behavior

SARIF reporter ignores `--quiet`. Full structure is always emitted.

## JUnit Reporter

### Mapping

| cfv concept | JUnit element |
|-------------|---------------|
| Report (file) | `<testcase>` |
| Issue (error) | `<failure>` |
| Issue (warning) | `<failure type="warning">` or omitted (configurable) |
| Notes | ignored |
| FixInfo | not representable — omitted |

### Test Case Structure

```xml
<testsuite name="config-file-validator" tests="51" failures="4">
  <testcase name="config.yml" classname="config.yml">
    <failure type="schema" message="&quot;8080&quot; is string, schema expects integer [schema/string-to-int]">
Line 5, Column 12: "8080" is string, schema expects integer
    </failure>
  </testcase>
  <testcase name="main.toml" classname="main.toml">
    <failure type="format" message="inconsistent indentation [format/indent]">
inconsistent indentation (tabs, expected 2 spaces)
    </failure>
  </testcase>
  <testcase name="app.json" classname="deploy/app.json" />
</testsuite>
```

### Format Warning Handling

By default, files with only format warnings do NOT count as failures in JUnit output. They appear as passing test cases. This matches the principle that format issues don't make a file invalid.

When `cfv format` is the subcommand, format warnings DO appear as failures (the subcommand's purpose is to surface them).

### Fix Info

JUnit cannot represent fix information. It is silently dropped.

## GitHub Reporter

### Output Format

GitHub workflow commands, one per issue:

```
::error file=config.yml,line=5,col=12,endLine=5,endColumn=18,title=schema/string-to-int::"8080" is string, schema expects integer
::error file=deploy/app.json,line=12,col=5,title=json/trailing-comma::trailing comma
::warning file=main.toml,title=format/indent::inconsistent indentation (tabs, expected 2 spaces)
::warning file=.env,title=format/spacing::spaces around "=" (expected no spaces)
```

### Severity Mapping

| cfv Severity | GitHub command |
|--------------|---------------|
| Error | `::error` |
| Warning | `::warning` |
| Note | `::notice` |

### Position Fields

Only included when non-zero:
- `line` — when `Issue.Line > 0`
- `col` — when `Issue.Column > 0`
- `endLine` — when `Issue.EndLine > 0`
- `endColumn` — when `Issue.EndColumn > 0`

### Fix Info

GitHub annotations cannot represent fixes. Fix info is silently dropped. The summary line (printed to stdout after all annotations) includes fix hints so developers know to run `--fix` locally.

### --quiet Behavior

GitHub reporter ignores `--quiet`. Annotations are always emitted (they're the point of this reporter).

## Post-Fix Output

When `--fix` is applied:

1. The fixer applies all edits with matching safety level (`--fix` = safe only, `--fix --unsafe` = safe + unsafe).
2. Fixed issues are removed from the report before it reaches the reporter.
3. An additional summary line is prepended: `Fixed N issues in M files`.
4. If issues remain after fixing, the reporter prints them normally.
5. If all issues are fixed, the reporter prints only the fix summary and exits 0.

### Example: Partial Fix

```
Fixed 2 issues in 2 files

  × config.yml:5:12 — "8080" is string, schema expects integer [schema/string-to-int]
  ✓ 50 files passed

Found 1 error
```

The trailing-comma issue was fixed, so it no longer appears.

### Example: Full Fix

```
Fixed 4 issues in 3 files
```

Exit code: 0. No issue lines printed.

### JSON Post-Fix

The JSON reporter includes a `fixed` summary field:

```json
{
  "fixed": {
    "issues": 2,
    "files": 2
  },
  "summary": {
    "total_files": 51,
    "files_with_issues": 1,
    "errors": 1,
    "warnings": 0,
    "fixable_errors": 0,
    "fixable_warnings": 0
  },
  "results": [ ... ]
}
```

`fixed` is omitted when `--fix` is not used.

## GroupBy Behavior

GroupBy applies to stdout and JSON reporters. Other reporters ignore it.

### Supported GroupBy Values

| Value | Groups by | Header format (stdout) |
|-------|-----------|----------------------|
| `directory` | Parent directory of each file | `deploy/:` |
| `type` | IssueType | `Syntax errors:`, `Schema errors:`, `Format issues:` |
| `severity` | Severity | `Errors:`, `Warnings:` |
| `pass-fail` | IsValid() | `Failed:`, `Passed:` (v2 compat) |

### Sorting Within Groups

Files within a group are sorted by filesystem walk order (the order they appear in the input `[]Report` slice). Issues within a file are sorted by line number, then column number.

### Empty Groups

Groups with no matching items are omitted from output.

## Backward Compatibility

### Breaking Changes (v3)

| Change | Impact |
|--------|--------|
| `Report` struct fields removed/replaced | All code referencing `IsValid`, `ValidationError`, `ValidationErrors`, `Warnings`, `ErrorType`, `IsQuiet`, `StartLine`, `StartColumn`, `ErrorLines`, `ErrorColumns` must be updated |
| `IsValid` is now a method, not a field | Code doing `report.IsValid = true` fails to compile |
| JSON output shape changed | Consumers parsing v2 JSON output break |
| JUnit format-warning handling | Files that were `<failure>` in v2 may become passing in v3 |

### Unchanged

| Aspect | Details |
|--------|---------|
| `Reporter` interface signature | `Print(reports []Report) error` — same signature, new `Report` type |
| Reporter construction | `NewFooReporter(outputDest string, isQuiet bool)` pattern preserved |
| File output mechanism | `outputBytesToFile()` utility unchanged |
| CLI flag `--reporter` | Same flag name, same reporter name strings |

### Migration Path

v2 users upgrading to v3:
1. Update import path from `v2` to `v3`.
2. Replace `report.IsValid` field access with `report.IsValid()` method call.
3. Replace `report.ValidationError` / `report.ValidationErrors` with iteration over `report.Issues`.
4. Replace `report.Warnings` with filtering `report.Issues` by `Severity == SeverityWarning`.
5. Replace `report.StartLine` / `report.StartColumn` with `issue.Line` / `issue.Column` on individual issues.
6. Update JSON output consumers to the new schema.

## Open Design Points (Not in This Spec)

- `--verbose` flag behavior (reserved, deferred past v3.0).
- Custom reporter plugin interface (load reporters from shared libraries or WASM).
- Streaming output (print issues as they're found, not after all files processed).
- `--format` flag to select output format without changing reporter (e.g., `--format=compact`).
