# cfv 3.0 CLI Architecture Spec

Status: Draft
Date: 2026-06-29
Authors: config-file-validator maintainers

## Overview

cfv 3.0 introduces a subcommand model. The binary does two things — validates syntax/schema (`check`) and reports formatting issues (`format`). Running `cfv` without a subcommand runs both.

The CLI stays on Go's `flag` package with a thin subcommand router. No cobra, no urfavecli, no new dependencies.

## Command Grammar

```
cfv [global-flags] [subcommand] [subcommand-flags] [paths...]
```

If no subcommand is given, `cfv` runs in **combined mode** (check + format). If no paths are given, the default is `.` (current directory).

### Subcommands

| Command       | Behavior                                                    |
|---------------|-------------------------------------------------------------|
| (none)        | Run check + format. Report all issues.                      |
| `check`      | Syntax validation + schema validation only.                 |
| `format`     | Report formatting issues only. Does not write unless --fix. |
| `version`    | Print version and exit.                                     |
| `help`       | Print help and exit. `help check` prints check help.        |

### Examples

```
cfv .                           # check + format on current dir
cfv check .                     # syntax/schema only
cfv format .                    # formatting issues only
cfv format --fix .              # rewrite files to fix formatting
cfv --fix .                     # fix both syntax/schema and formatting (safe fixes only)
cfv --fix --unsafe .            # fix everything including unsafe fixes
cfv check --reporter json .     # check with JSON output
cfv check --file-types json -   # validate JSON from stdin
cfv --version                   # print version
cfv version                     # print version
```

## Dispatch Algorithm

```
func main():
    args = os.Args[1:]

    // Phase 1: Peel global flags from the front.
    globalFlags = new FlagSet("cfv", ContinueOnError)
    register all global flags on globalFlags
    globalFlags.Parse(args)
    remaining = globalFlags.Args()  // everything after flags

    // Phase 2: Handle --version and --help at the global level.
    if globalFlags.version:
        printVersion(); exit(0)
    if globalFlags.help or len(remaining) == 0 and no paths implied:
        printUsage(); exit(0)

    // Phase 3: Determine subcommand vs path.
    subcommand = ""
    subArgs = remaining

    if len(remaining) > 0:
        first = remaining[0]
        switch first:
        case "check", "format", "version", "help":
            subcommand = first
            subArgs = remaining[1:]
        case starts with "-":
            // Flag after global flags — this is an error.
            // Global flags must come before the subcommand.
            die("unknown flag %q; flags must precede the subcommand", first)
        default:
            // Not a known subcommand — treat as a path (combined mode).
            subcommand = ""
            subArgs = remaining

    // Phase 4: Handle meta-subcommands.
    if subcommand == "version":
        printVersion(); exit(0)
    if subcommand == "help":
        if len(subArgs) > 0 and subArgs[0] in ["check", "format"]:
            printSubcommandHelp(subArgs[0]); exit(0)
        printUsage(); exit(0)

    // Phase 5: Parse subcommand-specific flags.
    subcmdFlags = new FlagSet(subcommand, ContinueOnError)
    register subcommand-specific flags on subcmdFlags
    subcmdFlags.Parse(subArgs)
    paths = subcmdFlags.Args()

    // Phase 6: Default paths.
    if len(paths) == 0:
        paths = ["."]

    // Phase 7: Resolve config (.cfv.toml, env vars).
    config = resolveConfig(globalFlags, subcmdFlags)

    // Phase 8: Dispatch.
    switch subcommand:
    case "check":
        exit(runCheck(config, paths))
    case "format":
        exit(runFormat(config, paths))
    case "":
        exit(runCombined(config, paths))
```

### How `cfv .` Composes Check + Format

Combined mode runs both pipelines sequentially on the same file set:

1. Build the finder (same for both).
2. Walk the filesystem once. For each file:
   a. Run syntax validation (check).
   b. If syntax is valid, run schema validation (check).
   c. Run format check (format).
3. Collect all reports from both pipelines into a single report list.
4. Pass the combined report list to the reporter.
5. Exit code: 0 if zero issues from either pipeline. 1 if any issue found. 2 on tool error.

When `--fix` is active in combined mode:
1. Run check fixes first (syntax/schema corrections).
2. Re-read the corrected content.
3. Run format fixes on the corrected content.
4. Write the final result once.

This ordering prevents format fixes from being invalidated by syntax fixes.

## Subcommand Router — Go Implementation Sketch

```go
package main

import (
    "flag"
    "fmt"
    "os"
    "strings"
)

// knownSubcommands lists valid subcommands for error suggestions.
var knownSubcommands = []string{"check", "format", "version", "help"}

func main() {
    os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
    // --- Global flags ---
    global := flag.NewFlagSet("cfv", flag.ContinueOnError)
    var g GlobalFlags
    registerGlobalFlags(global, &g)

    if err := global.Parse(args); err != nil {
        if err == flag.ErrHelp {
            return 0
        }
        return 2
    }

    remaining := global.Args()

    // --- Version shortcut ---
    if g.Version {
        printVersion()
        return 0
    }

    // --- Determine subcommand ---
    var subcmd string
    var subArgs []string

    if len(remaining) > 0 {
        switch remaining[0] {
        case "check", "format":
            subcmd = remaining[0]
            subArgs = remaining[1:]
        case "version":
            printVersion()
            return 0
        case "help":
            return handleHelp(remaining[1:])
        default:
            if strings.HasPrefix(remaining[0], "-") {
                fmt.Fprintf(os.Stderr, "cfv: unknown flag %q after global flags\n", remaining[0])
                return 2
            }
            // Treat as path — combined mode.
            subcmd = ""
            subArgs = remaining
        }
    } else {
        // No args at all — combined mode on "."
        subcmd = ""
        subArgs = nil
    }

    // --- Parse subcommand flags ---
    sub := flag.NewFlagSet(subcmd, flag.ContinueOnError)
    var sf SubcommandFlags
    registerSubcommandFlags(sub, &sf, subcmd)

    if err := sub.Parse(subArgs); err != nil {
        if err == flag.ErrHelp {
            return 0
        }
        return 2
    }

    paths := sub.Args()
    if len(paths) == 0 {
        paths = []string{"."}
    }

    // --- Resolve config ---
    config, err := resolveConfig(g, sf, subcmd)
    if err != nil {
        fmt.Fprintf(os.Stderr, "cfv: %v\n", err)
        return 2
    }

    // --- Dispatch ---
    switch subcmd {
    case "check":
        return runCheck(config, paths)
    case "format":
        return runFormat(config, paths)
    default:
        return runCombined(config, paths)
    }
}

func handleHelp(args []string) int {
    if len(args) > 0 {
        switch args[0] {
        case "check":
            printCheckHelp()
            return 0
        case "format":
            printFormatHelp()
            return 0
        default:
            suggestion := suggestSubcommand(args[0])
            fmt.Fprintf(os.Stderr, "cfv: unknown command %q%s\n", args[0], suggestion)
            return 2
        }
    }
    printUsage()
    return 0
}

func suggestSubcommand(input string) string {
    for _, cmd := range knownSubcommands {
        if levenshtein(input, cmd) <= 2 {
            return fmt.Sprintf(". Did you mean %q?", cmd)
        }
    }
    return ""
}
```

### Flag Registration Pattern

```go
type GlobalFlags struct {
    Version         bool
    Quiet           bool
    Config          string
    NoConfig        bool
    Gitignore       bool
    IgnoreFiles     []string // repeatable
    ExcludeDirs     []string // repeatable
    ExcludeFileTypes []string // repeatable
    FileTypes       []string // repeatable
    TypeMap         []string // repeatable
    SchemaMap       []string // repeatable
    SchemaStore     string
    SchemaStorePath string
    Depth           int
    GroupBy         string
    Globbing        bool
    RequireSchema   bool
    NoSchema        bool
    Reporters       []string // repeatable
}

type SubcommandFlags struct {
    Fix    bool
    Unsafe bool
    Diff   bool // format only
}

func registerGlobalFlags(fs *flag.FlagSet, g *GlobalFlags) {
    fs.BoolVar(&g.Version, "version", false, "")
    fs.BoolVar(&g.Quiet, "quiet", false, "")
    fs.StringVar(&g.Config, "config", "", "")
    fs.BoolVar(&g.NoConfig, "no-config", false, "")
    fs.BoolVar(&g.Gitignore, "gitignore", false, "")
    // ... repeatable flags use a custom Value implementation
    fs.Var(&repeatableString{&g.IgnoreFiles}, "ignore-file", "")
    fs.Var(&repeatableString{&g.ExcludeDirs}, "exclude-dirs", "")
    fs.Var(&repeatableString{&g.ExcludeFileTypes}, "exclude-file-types", "")
    fs.Var(&repeatableString{&g.FileTypes}, "file-types", "")
    fs.Var(&repeatableString{&g.TypeMap}, "type-map", "")
    fs.Var(&repeatableString{&g.SchemaMap}, "schema-map", "")
    fs.StringVar(&g.SchemaStore, "schemastore", "auto", "")
    fs.StringVar(&g.SchemaStorePath, "schemastore-path", "", "")
    fs.IntVar(&g.Depth, "depth", -1, "")
    fs.StringVar(&g.GroupBy, "groupby", "", "")
    fs.BoolVar(&g.Globbing, "globbing", false, "")
    fs.BoolVar(&g.RequireSchema, "require-schema", false, "")
    fs.BoolVar(&g.NoSchema, "no-schema", false, "")
    fs.Var(&repeatableString{&g.Reporters}, "reporter", "")
}

func registerSubcommandFlags(fs *flag.FlagSet, sf *SubcommandFlags, subcmd string) {
    fs.BoolVar(&sf.Fix, "fix", false, "")
    fs.BoolVar(&sf.Unsafe, "unsafe", false, "")
    if subcmd == "format" || subcmd == "" {
        fs.BoolVar(&sf.Diff, "diff", false, "")
    }
}
```

## Complete Flag Table

### Global Flags

| Flag | Type | Default | Env Var | Config Key | Description |
|------|------|---------|---------|------------|-------------|
| `--version` | bool | false | — | — | Print version and exit |
| `--quiet` | bool | false | `CFV_QUIET` | `quiet` | Suppress stdout when writing to file |
| `--config` | string | `.cfv.toml` | `CFV_CONFIG` | — | Path to config file |
| `--no-config` | bool | false | `CFV_NO_CONFIG` | — | Ignore config file |
| `--gitignore` | bool | false | `CFV_GITIGNORE` | `gitignore` | Respect .gitignore patterns |
| `--ignore-file` | string[] | [] | `CFV_IGNORE_FILES` | `ignore-files` | Additional ignore files (repeatable) |
| `--exclude-dirs` | string[] | [] | `CFV_EXCLUDE_DIRS` | `exclude-dirs` | Directories to skip (repeatable) |
| `--exclude-file-types` | string[] | [] | `CFV_EXCLUDE_FILE_TYPES` | `exclude-file-types` | File types to skip (repeatable) |
| `--file-types` | string[] | [] | `CFV_FILE_TYPES` | `file-types` | Only validate these types (repeatable) |
| `--type-map` | string[] | [] | `CFV_TYPE_MAP` | `type-map` | Extension-to-type overrides (repeatable) |
| `--schema-map` | string[] | [] | `CFV_SCHEMA_MAP` | `schema-map` | File-to-schema mappings (repeatable) |
| `--schemastore` | string | `"auto"` | `CFV_SCHEMASTORE` | `schemastore` | SchemaStore mode: auto, off, only |
| `--schemastore-path` | string | `""` | `CFV_SCHEMASTORE_PATH` | `schemastore-path` | Local SchemaStore catalog path |
| `--depth` | int | -1 (unlimited) | `CFV_DEPTH` | `depth` | Max directory recursion depth |
| `--groupby` | string | `""` | `CFV_GROUPBY` | `groupby` | Group output: directory, pass-fail, filetype |
| `--globbing` | bool | false | `CFV_GLOBBING` | `globbing` | Treat paths as globs |
| `--require-schema` | bool | false | `CFV_REQUIRE_SCHEMA` | `require-schema` | Fail files without a schema |
| `--no-schema` | bool | false | `CFV_NO_SCHEMA` | `no-schema` | Skip all schema validation |
| `--reporter` | string[] | `["stdout"]` | `CFV_REPORTER` | `reporter` | Output format (repeatable). Syntax: `type` or `type:path` |

### Subcommand Flags

| Flag | Type | Default | Subcommands | Env Var | Config Key | Description |
|------|------|---------|-------------|---------|------------|-------------|
| `--fix` | bool | false | check, format, (bare) | `CFV_FIX` | `fix` | Write safe fixes to disk |
| `--unsafe` | bool | false | check, format, (bare) | `CFV_UNSAFE` | `unsafe` | Include unsafe fixes (requires --fix) |
| `--diff` | bool | false | format, (bare) | `CFV_DIFF` | `diff` | Show unified diff of formatting changes |

### Positional Arguments

| Position | Meaning | Default |
|----------|---------|---------|
| After all flags | Filesystem paths to validate. Directories are recursed. `-` means stdin. | `.` |

## Flag Resolution Order (Highest Wins)

1. CLI flag (explicit on command line)
2. Environment variable
3. `.cfv.toml` config file
4. Built-in default

A flag is "set" if provided on the command line. Unset flags fall through to env, then config, then default. The `--no-config` flag skips step 3.

### Env Var Parsing Rules

| Type | Parsing |
|------|---------|
| bool | `"true"`, `"1"`, `"yes"` → true. Anything else → false. |
| int | `strconv.Atoi`. Invalid → error (exit 2). |
| string | Used as-is. |
| string[] | Comma-separated. `CFV_EXCLUDE_DIRS=node_modules,dist` → `["node_modules", "dist"]` |

## Exit Codes

| Code | Meaning | When |
|------|---------|------|
| 0 | Success | No issues found in any file. |
| 1 | Issues found | At least one file has a validation or formatting issue. |
| 2 | Tool error | Bad flags, unreadable config, IO error, panic recovery. |

Exit code semantics are identical across all subcommands and combined mode. The highest-severity code wins (2 > 1 > 0).

### Exit Code Examples

```
cfv check .            → 0 if all files valid, 1 if any invalid
cfv format .           → 0 if all files formatted, 1 if any unformatted
cfv .                  → 0 if check AND format both pass, 1 if either has issues
cfv check bad-flag     → 2 (unknown flag)
cfv --config /x .      → 2 (config file not found)
cfv check --fix .      → 0 after fixing (issues were resolved), 1 if some unfixable
```

## Stdin Behavior

When a path argument is `-`, cfv reads from stdin instead of the filesystem.

**Constraints:**
- Exactly one `-` allowed. Multiple `-` arguments → error (exit 2).
- `--file-types` is required with stdin. Without it, cfv cannot detect the format.
- If `--file-types` specifies multiple types, stdin is validated against all of them (useful for polyglot files, but unusual).
- Stdin is fully buffered into memory before validation.
- `--fix` with stdin writes the fixed content to stdout (does not write to a file).
- `--diff` with stdin shows the diff to stderr (stdout is reserved for fixed content when `--fix` is active).

**Error:**
```
$ cfv check -
cfv: stdin requires --file-types to detect format

$ cfv check --file-types json - -
cfv: stdin (-) can only be specified once
```

## Stdout/Stderr Contract

| Stream | Content |
|--------|---------|
| stdout | Reporter output (reports, JSON, JUnit, SARIF). Fixed file content when using `--fix` with stdin. |
| stderr | Errors, warnings, progress (if ever added). Help text on error. |

When `--quiet` is set and at least one reporter writes to a file, stdout is suppressed. Stderr is never suppressed.

## Error Messages for Common Mistakes

```
$ cfv cheeck .
cfv: unknown command "cheeck". Did you mean "check"?

$ cfv --fix
cfv: no paths specified (default is ".")
# This actually works — defaults to ".". This error only fires if we
# somehow get zero paths after resolution, which shouldn't happen.

$ cfv check --diff .
cfv: --diff is only valid with "format" (or the bare command)

$ cfv format --unsafe .
cfv: --unsafe requires --fix

$ cfv --require-schema --no-schema .
cfv: --require-schema and --no-schema are mutually exclusive

$ cfv check --file-types notaformat .
cfv: unknown file type "notaformat". Run "cfv help" for supported types.

$ cfv --reporter badformat .
cfv: unknown reporter "badformat". Supported: stdout, json, junit, sarif, github
```

All error messages:
- Start with `cfv:` (lowercase, no "Error:" prefix).
- Go to stderr.
- Are followed by exit code 2.
- Include actionable guidance when possible.

## Config File Interaction

The `.cfv.toml` config file provides defaults. CLI flags and env vars override it.

```toml
# .cfv.toml
gitignore = true
depth = 5
exclude-dirs = ["node_modules", ".git", "vendor"]
reporter = ["stdout", "json:reports/cfv.json"]
fix = false

[format]
diff = true

[check]
require-schema = true
```

Section headers (`[format]`, `[check]`) scope config to a subcommand. Top-level keys apply globally. Subcommand sections override top-level for that subcommand.

Resolution for `cfv format .`:
1. Start with built-in defaults.
2. Apply top-level config keys.
3. Apply `[format]` section (overrides top-level for conflicting keys).
4. Apply env vars (override config).
5. Apply CLI flags (override everything).

## Reporter Flag Syntax

```
--reporter type
--reporter type:path
```

Multiple reporters are supported by repeating the flag:

```
cfv --reporter stdout --reporter json:out.json --reporter junit:report.xml .
```

If no `--reporter` is specified, defaults to `stdout`.

Supported reporter types: `stdout`, `json`, `junit`, `sarif`, `github`.

When a reporter has a path (e.g., `json:out.json`), output goes to that file. The `stdout` reporter always writes to stdout (path is ignored if given).

## How Global Flags Are Inherited

Global flags are parsed once at the top level. The resulting `GlobalFlags` struct is passed to the subcommand handler. There is no re-parsing or flag merging — the subcommand handler receives the already-resolved global state.

```go
func runCheck(config ResolvedConfig, paths []string) int {
    // config contains both global and subcommand-specific settings
    // already resolved through the priority chain.
}
```

The `ResolvedConfig` struct is the single source of truth after resolution:

```go
type ResolvedConfig struct {
    // From global flags
    Quiet           bool
    Gitignore       bool
    IgnoreFiles     []string
    ExcludeDirs     []string
    ExcludeFileTypes []string
    FileTypes       []string
    TypeMap         map[string]string
    SchemaMap       map[string]string
    SchemaStore     string
    SchemaStorePath string
    Depth           int
    GroupBy         string
    Globbing        bool
    RequireSchema   bool
    NoSchema        bool
    Reporters       []ReporterConfig

    // From subcommand flags
    Fix    bool
    Unsafe bool
    Diff   bool

    // Derived
    Stdin  bool // true if paths contains "-"
}
```

## Validation Rules (Checked Before Dispatch)

These are checked after full resolution and before any filesystem work:

| Rule | Error |
|------|-------|
| `--unsafe` without `--fix` | `--unsafe requires --fix` |
| `--require-schema` with `--no-schema` | `--require-schema and --no-schema are mutually exclusive` |
| `--diff` on `check` subcommand | `--diff is only valid with "format" (or the bare command)` |
| stdin (`-`) without `--file-types` | `stdin requires --file-types to detect format` |
| stdin (`-`) appears multiple times | `stdin (-) can only be specified once` |
| `--depth` < -1 | `--depth must be -1 (unlimited) or non-negative` |
| Unknown reporter type | `unknown reporter "X". Supported: stdout, json, junit, sarif, github` |
| Unknown file type in `--file-types` | `unknown file type "X". Run "cfv help" for supported types.` |
| Config file specified but not found | `config file not found: /path/to/.cfv.toml` |

## Help Output Format

```
$ cfv --help
Config File Validator - validate config files across 18 formats

Usage:
  cfv [flags] [paths...]              Validate syntax, schema, and formatting
  cfv check [flags] [paths...]        Validate syntax and schema only
  cfv format [flags] [paths...]       Check formatting only

Flags:
  --reporter type[:path]   Output format (repeatable) [stdout]
  --quiet                  Suppress stdout when writing to file
  --config path            Config file path [.cfv.toml]
  --no-config              Ignore config file
  --gitignore              Respect .gitignore patterns
  --ignore-file path       Additional ignore file (repeatable)
  --exclude-dirs dirs      Directories to skip (repeatable)
  --exclude-file-types t   File types to skip (repeatable)
  --file-types types       Only validate these types (repeatable)
  --type-map ext=type      Extension-to-type override (repeatable)
  --schema-map file=url    File-to-schema mapping (repeatable)
  --schemastore mode       SchemaStore mode: auto, off, only [auto]
  --schemastore-path path  Local SchemaStore catalog path
  --depth n                Max recursion depth [-1 unlimited]
  --groupby key            Group output: directory, pass-fail, filetype
  --globbing               Treat paths as globs
  --require-schema         Fail files without a schema
  --no-schema              Skip all schema validation
  --fix                    Write safe fixes to disk
  --unsafe                 Include unsafe fixes (requires --fix)
  --diff                   Show unified diff of format changes
  --version                Print version

Run "cfv help <command>" for subcommand details.
```

```
$ cfv help check
Validate syntax and schema of config files.

Usage:
  cfv check [flags] [paths...]

Flags:
  --fix                    Fix syntax/schema issues and write to disk
  --unsafe                 Include unsafe fixes (requires --fix)

All global flags are also accepted. Run "cfv --help" for the full list.

Exit codes:
  0  All files valid
  1  One or more files invalid
  2  Tool error
```

## Version Output

```
$ cfv --version
cfv 3.0.0
```

Single line. No extra decoration. Parseable by scripts.

## Binary Name

The binary is renamed from `validator` to `cfv` in 3.0. The module path remains `github.com/Boeing/config-file-validator/v3`. The binary source moves from `cmd/validator/` to `cmd/cfv/`.

```
go install github.com/Boeing/config-file-validator/v3/cmd/cfv@latest
```

## Summary of Breaking Changes from 2.x

| 2.x Behavior | 3.0 Behavior | Migration |
|---------------|--------------|-----------|
| Binary named `validator` | Binary named `cfv` | Rename in scripts |
| Flat flags only | Subcommand model | `validator .` → `cfv check .` (or just `cfv .`) |
| `--reporter json` | `--reporter json` (unchanged) | No change |
| Exit 1 for any error | Exit 1 = issues, Exit 2 = tool error | Update CI scripts checking exit codes |
| No `--fix` | `--fix` writes corrections | Opt-in, no migration needed |
| No format checking | `cfv format` reports formatting | New feature, no migration needed |
