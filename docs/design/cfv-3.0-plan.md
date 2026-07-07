# cfv 3.0 — Implementation Plan

## Current State — Resume Here

**Branch**: `feat/3.0`
**Last updated**: 2026-07-07
**Next task**: Phase 2, Task 8 — Final stress test + Opus review

### What's done

**Phase 1 ✅ Complete**
- Binary renamed `validator` → `cfv`. Module path v2→v3.
- Subcommand router: `cfv check`, `cfv format`, `cfv version`, `cfv help`
- `cfv check .` = identical to old `validator .`
- `cmd/validator/` deleted. No shim.
- All 18 website docs updated.
- Opus review done, 4 architectural issues fixed.

**Phase 2 — In progress**
- ✅ `Formatter` interface, `Options` struct (with `QuoteStyle`), `IsFormatted` helper (`pkg/formatter/`)
- ✅ `Report` struct refactored to v3: `Status` enum (Pass/Fail/Unformatted), `Issues []Issue`. All 5 reporters updated.
- ✅ JSON formatter (`pkg/formatter/jsonfmt/`) — 11 fixture tests, idempotency, fuzz, all options
- ✅ `cfv format .` wired — reports unformatted files with `~` symbol, exit 1
- ✅ `cfv format --fix .` wired — atomic writes (temp + rename), exit 0
- ✅ `Formatter` field added to `FileType`. Registered in `pkg/filetype/formatters.go`.
- ✅ Parallel worker pool (`runtime.NumCPU()`) in `pkg/cli/format.go`
- ✅ Code review fixes (Tasks 4.5.1–4.5.6, 4.5.9–4.5.11) — ALL COMPLETE
- ✅ YAML formatter (`pkg/formatter/yamlfmt/`) — goccy/go-yaml, token-preserving AST
- ✅ YAML sort-keys (depth-based `AddColumn` reindent, `SortStableFunc` on mapping values)
- ✅ YAML quote-style (double/single/preserve on values only, keys untouched)
- ✅ HCL formatter (`pkg/formatter/hclfmt/`) — hclwrite.Format wrapper
- ✅ `.cfv.toml` `[format]` + `[format.<type>]` config parser with JSON Schema validation
- ✅ CLI flags: `--indent`, `--use-tabs`, `--sort-keys`, `--no-sort-keys`, `--line-ending`, `--max-line-width`, `--quote-style`, `--diff`
- ✅ Resolution cascade: CLI > per-format config > global config > format-specific defaults
- ✅ `FormatOptionsFunc` — per-format options resolution in `cli.Format`
- ✅ `--diff` flag — unified diff output via go-difflib
- ✅ CLI flag validation (mutual exclusion, range checks, enum checks)
- ✅ Library swap: `gopkg.in/yaml.v3` → `github.com/goccy/go-yaml` (formatter + validator + generator)
- ✅ Output validation: fail-fast on unsupported YAML constructs
- ✅ Stress tested: 17M fuzz executions, 1830-file smoke test, all edge cases handled
- ✅ XML formatter (`pkg/formatter/xmlfmt/`) — go-xmlfmt/xmlfmt, 96.3% coverage
- ✅ Properties formatter (`pkg/formatter/propfmt/`) — magiconair/properties
- ✅ INI formatter (`pkg/formatter/inifmt/`) — gopkg.in/ini.v1, 92.3% coverage
- ✅ ENV formatter (`pkg/formatter/envfmt/`) — custom line-oriented, 92.3% coverage
- ✅ TOML formatter (`pkg/formatter/tomlfmt/`) — pelletier/go-toml/v2 (no comment preservation)

### What's next

**Task 7**: TOML, ENV, INI, XML, Properties formatters

Design needed before implementation (same lesson as YAML — prototype first):
- TOML: evaluate `pelletier/go-toml/v2` unstable.Parser vs goccy-style token approach
- ENV: custom line-oriented (simplest — key=value with comment preservation)
- INI: `gopkg.in/ini.v1` or custom
- XML: `go-xmlfmt/xmlfmt` (new dep, MIT, zero transitive)
- Properties: `magiconair/properties` (already a dep)

**Task 8**: Final stress test + Opus review

### How to start the next session

```
cd /Users/se456c/src/github.com/boeing/config-file-validator
git checkout feat/3.0
go test ./cmd/cfv/ ./pkg/... # verify all green
```

Then say "let's keep going" — next task is Task 8 (Final stress test + Opus review).

### Pipeline state

```
go vet ./...           ✅
gofmt -s -l -e .       ✅
golangci-lint run      ✅ 0 issues
go test ./...          ✅ all pass
fuzz (60s each)        ✅ 17M executions, 0 failures
coverage               ✅ 93.4%
```

### Release strategy

v3 ships as release candidates while v2 continues receiving patches on main.

```
main         → v2.4.x patches (security, critical bugs like SARIF schema fix)
feat/3.0     → v3.0.0-rc1, rc2, ... (community testing)
```

When v3.0.0 is ready:
1. Tag `v3.0.0` on `feat/3.0`
2. Merge `feat/3.0` → `main`
3. Cut `v2` branch from last v2 tag for ongoing maintenance
4. Homebrew/winget point to v3 as default

Go modules make this seamless:
- `go install .../v2/cmd/validator@latest` → still works, gets patches
- `go install .../v3/cmd/cfv@v3.0.0-rc1` → opt-in RC testing
- `@latest` never resolves to pre-release — safe for production

No forced migration. Both versions alive simultaneously. RCs bake as long
as needed — no pressure to rush.

### Key decisions made this session

1. **goccy/go-yaml over yaml.v3** — eliminates 4 yaml.v3 limitations (!!merge, folded joining, emoji quoting, comment instability). Token-preserving AST gives full control.
2. **Depth-based reindent via AddColumn** — not column-ratio math. Correctly handles sequences nested in mappings.
3. **Fail-fast on unsupported constructs** — pathological YAML (bare scalars, complex keys, null-as-key) gets a clear error, not silent corruption or silent passthrough.
4. **Fuzz tests verify no-panic + valid-output** — not idempotency on arbitrary input. Idempotency is tested via fixtures on real config patterns.
5. **Config cascade proven exhaustively** — 46-command txtar test covers every key × every layer.
6. **Per-format defaults differ** — JSON defaults sort-keys=true, YAML defaults sort-keys=false. Cascade handles this correctly.
7. **quote-style only affects values** — keys are never modified (prettier/yamlfmt convention).
8. **--diff and --fix are mutually exclusive** — clear error if both provided.
9. **editorconfig deferred to 3.1** — requires per-file resolution (not per-format), adds complexity to the cascade.
10. **Summary line + pass suppression deferred to Phase 4** — needs a distinct reporter mode for `cfv .` vs `cfv check`.

### Bugs found and fixed this session

| Bug | Severity | Fix |
|-----|----------|-----|
| Parse errors reported as pass | High | formatFile returns nil for skip |
| Hardcoded "run cfv format --fix" in library | Medium | Generic message |
| Unnecessary mutex in worker pool | Low | Removed |
| StatusUnformatted counted as Failed | Medium | Separate Unformatted counter |
| JUnit drops unformatted files | Medium | Switch case added |
| resolveConfig panics on nil schema fields | Medium | Nil-safe checks |
| Config auto-discovery not wired to format resolver | High | Pass formatCfg through resolvedConfig |
| --sort-keys/--no-sort-keys ordering | Medium | Mutually exclusive error |
| --fix --diff silent ignore | Medium | Mutually exclusive error |
| --indent out of range accepted | Medium | Range validation |
| --line-ending=banana accepted | Medium | Enum validation |
| --quote-style=banana accepted | Medium | Enum validation |
| quote-style applied to keys | Medium | Key/value awareness in normalizeNode |
| Multi-doc duplicate keys missed by validator | Medium | goccy parser catches all docs |
| Document end marker dropped | Low | goccy preserves natively |
| yaml.v3 !!merge tag injection | Medium | Eliminated by library swap |
| yaml.v3 foot comment instability | High | Eliminated by library swap |
| Flow-style mapping panic on AddColumn | High | Skip flow-style in reindent |
| Stale known_files_gen.go in root dir | Low | Removed |

### What's next

**Task 4.5**: Code review fixes (prerequisite before YAML formatter)

Addresses 11 issues from the 2026-06-30 deep review of Tasks 1–4. Ordered by
dependency — later tasks assume earlier ones are done.

---

#### 4.5.1 — Fix parse-error-as-pass bug (format.go)

**Problem**: When `formatter.Format()` returns an error (unparseable file) or
`os.ReadFile()` fails (non-symlink), `formatFile` returns `StatusPass`. This
inflates the pass count and hides broken files.

**Fix**:
- Introduce a sentinel: files the format pipeline cannot process should be
  *excluded* from the report entirely (nil report or a `StatusSkipped` that the
  caller filters out before sending to reporters).
- Simplest approach: return a `reporter.Report` with a new field
  `Skipped bool` — the `Format()` loop filters these out before building the
  `reports` slice. This avoids adding a fourth Status value that all 5
  reporters must handle.
- For broken symlinks, keep the current `StatusFail` behavior (that's
  actionable for the user).

**Files**: `pkg/cli/format.go`

**Tests**: Add a txtar test with an unparseable JSON file — `cfv format .`
should not list it as ✓. Add a test with an unreadable file (chmod 000) —
same expectation.

---

#### 4.5.2 — Extract normalizeLineEndings to shared package

**Problem**: `normalizeLineEndings` is defined in `jsonfmt/json.go`. Every
future formatter needs it.

**Fix**:
- Move to `pkg/formatter/lineendings.go` as an exported function:
  `func NormalizeLineEndings(data []byte, ending LineEnding) []byte`
- Update `jsonfmt` to call `formatter.NormalizeLineEndings(...)`.

**Files**: `pkg/formatter/lineendings.go` (new), `pkg/formatter/jsonfmt/json.go`

**Tests**: Unit test in `pkg/formatter/lineendings_test.go` covering LF
pass-through, CRLF conversion, and already-CRLF input.

---

#### 4.5.3 — Remove hardcoded "run cfv format --fix" message from pkg/cli

**Problem**: `pkg/cli/format.go` hardcodes
`"needs formatting (run cfv format --fix to rewrite)"`. This couples the
library to the CLI binary name. Library consumers get incorrect instructions.

**Fix**:
- Change the issue message to generic text: `"file is not formatted"`
- Move the actionable hint to the *summary line* (Task 4.5.7) which is
  emitted by the CLI entrypoint/reporter, not the engine.

**Files**: `pkg/cli/format.go`

**Tests**: Update any test that asserts on the old message string.

---

#### 4.5.4 — Remove unnecessary mutex in worker pool

**Problem**: Each goroutine writes to `results[idx]` — a unique index. The
mutex is pure overhead.

**Fix**:
- Remove `var mu sync.Mutex` and the `mu.Lock()`/`mu.Unlock()` calls.
- Write directly: `results[idx] = r` (the slice is pre-allocated, never
  resized; no two goroutines share an index).
- Also remove the `idx` field from the `result` struct since we're already
  indexing by position.

**Files**: `pkg/cli/format.go`

**Tests**: Existing tests + race detector (`go test -race ./pkg/cli/...`)
confirm correctness.

---

#### 4.5.5 — Distinguish "unformatted" from "failed" in summary counts

**Problem**: `createStdoutReport` counts `StatusUnformatted` under
`Summary.Failed`. The plan says format issues are warnings, not errors. Users
see "3 failed" when the real message should be "3 unformatted."

**Fix**:
- Add an `Unformatted int` field to the `summary` struct.
- `StatusUnformatted` increments `Unformatted`, not `Failed`.
- Update `PrintGroupStdout` summary line to include unformatted:
  `"N succeeded, M failed, P unformatted"`
- JSON reporter's summary object gets the same field.
- When unformatted is 0, omit it from the line (backwards compat with check).

**Files**: `pkg/reporter/stdout_reporter.go`, `pkg/reporter/json_reporter.go`,
`pkg/reporter/reporter.go` (summary struct if it's shared)

**Tests**: Update reporter tests and golden files to reflect the new summary
shape.

---

#### 4.5.6 — Fix JUnit reporter for unformatted files

**Problem**: JUnit only emits `TestcaseFailure` for `StatusFail`.
`StatusUnformatted` files appear as passing test cases — contradicting the
exit code.

**Fix**:
- When `Status == StatusUnformatted`, emit a `TestcaseFailure` with
  `message="formatting"` (matching JUnit convention: test marked as failure).
- Alternatively, emit as a JUnit "warning" attribute if the CI system
  supports it — but most don't. Failure is the safe default.
- Increment `testErrors` for unformatted files too.

**Files**: `pkg/reporter/junit_reporter.go`

**Tests**: Add a test case with `StatusUnformatted` reports and verify the
JUnit XML contains a failure element.

---

#### 4.5.7 — Add summary line with fixable count

**Problem**: No summary line like `Found N issues (M fixable with --fix)`.
The plan and pitch both show this as a first-class feature.

**Fix**:
- After printing individual file reports in `createStdoutReport`, append a
  summary line. Logic:
  - Count total issues (errors + unformatted).
  - All `IssueTypeFormat` issues are fixable with `--fix`.
  - Emit: `"\nFound %d issues (%d fixable with --fix)\n"` when issues > 0.
  - Emit: `"\n✓ %d files passed\n"` when zero issues.
- Only emit when *not* in grouped mode and *not* quiet.
- The summary line should be emitted by the stdout reporter only (JSON/SARIF
  reporters have their own summary structure).

**Files**: `pkg/reporter/stdout_reporter.go`

**Tests**: Update stdout reporter tests and golden files.

---

#### 4.5.8 — Reduce noise: only show issues in format mode

**Problem**: `cfv format .` sends every file (including passes) to the
reporter. On a 500-file repo with 3 issues, 500 lines of output.

**Fix**:
- In `CLI.Format()`, before calling `printReports`, filter out `StatusPass`
  reports. Only send `StatusUnformatted` and `StatusFail` reports to the
  reporter pipeline.
- The summary line (Task 4.5.7) handles the "X files passed" information.
- The check subcommand (`CLI.Run()`) continues showing all files — that's
  its existing behavior and users expect it.
- Add a method or option to distinguish: `Format()` sets a flag indicating
  "issues-only" mode that `printReports` respects, OR just filter inline
  before the call.

**Files**: `pkg/cli/format.go`

**Tests**: txtar test: run `cfv format .` on a directory with 5 valid files
and 1 unformatted file — output should show only the 1 unformatted file
plus the summary line.

---

#### 4.5.9 — Decouple resolveConfig from schema fields for format subcommand

**Problem**: `parseFormatFlags` creates dummy `emptyStr`/`falseVal` pointers
for schema fields just so `resolveConfig` doesn't panic. This is a
maintenance trap.

**Fix**:
- Extract the shared portion of `resolveConfig` into a helper:
  `resolveBaseConfig(cfg *cfvConfig) (*resolvedConfig, error)` — handles
  reporters, search paths, finder opts, config file, gitignore.
- `resolveCheckConfig` adds schema-specific resolution on top.
- `resolveFormatConfig` calls `resolveBaseConfig` directly — no schema
  fields needed, no dummy pointers.
- This removes the fake fields from `parseFormatFlags`.

**Files**: `cmd/cfv/cfv.go`

**Tests**: Existing tests should continue passing. Add a test that
`parseFormatFlags` does not require schema-related flags at all.

---

#### 4.5.10 — Fix fuzz test validation mismatch (minor)

**Problem**: The JSON formatter uses `json.Valid()` (accepts fragments like
bare `true`, `42`, `"hello"`) but the fuzz test's `isValidJSON` uses
`stdjson.Unmarshal` which has slightly different semantics.

**Fix**:
- Change `isValidJSON` to use `json.Valid()` for consistency with the
  formatter. Or document why `Unmarshal` is intentionally stricter.

**Files**: `pkg/formatter/jsonfmt/json_test.go`

---

#### 4.5.11 — Improve fixture option dispatch (minor, forward-looking)

**Problem**: `TestFixtures` uses `if name == "tab_indent"` to select options.
Won't scale to 10+ formatters.

**Fix**:
- Adopt a convention: if a file `testdata/<name>.opts.json` exists alongside
  the `.input.*` file, parse it as `formatter.Options` override. Otherwise
  use defaults.
- Implement this in a shared test helper:
  `func LoadFixtureOptions(name string) formatter.Options`
- Update the JSON formatter test to use this helper. The `tab_indent` fixture
  gets a `tab_indent.opts.json` sidecar:
  ```json
  {"IndentStyle": 2, "IndentWidth": 0}
  ```
  (where `2` = `IndentTabs` enum value)
- Future formatters (YAML, TOML, etc.) inherit this pattern automatically.

**Files**: `pkg/formatter/testutil_test.go` (new shared helper),
`pkg/formatter/jsonfmt/json_test.go`,
`pkg/formatter/jsonfmt/testdata/tab_indent.opts.json` (new)

---

### Execution order

```
4.5.1  ✅ (parse-error-as-pass)      — correctness bug, fixed
4.5.2  ✅ (extract lineendings)      — shared helper for all formatters
4.5.3  ✅ (hardcoded message)        — decoupled from binary name
4.5.4  ✅ (remove mutex)             — trivial cleanup
4.5.5  ✅ (summary counts)           — Unformatted tracked separately from Failed
4.5.6  ✅ (JUnit unformatted)        — StatusUnformatted produces test failure
4.5.7  DEFERRED → Phase 4           — summary line belongs with `cfv .` unified command
4.5.8  DEFERRED → Phase 4           — noise reduction belongs with `cfv .` unified command
4.5.9  ✅ (resolveConfig split)      — resolveBaseConfig + resolveCheckConfig + resolveFormatConfig
4.5.10 ✅ (fuzz validation)          — already uses stdjson.Valid (consistent with formatter)
4.5.11 ✅ (fixture options)          — LoadFixtureOptions in pkg/formatter/fixture_opts.go
```

**Why 4.5.7 and 4.5.8 were deferred**: The summary line and pass-line
suppression touch the shared `StdoutReporter` which is also used by
`cfv check`. Changing `cfv check` output breaks v2 backward compatibility
(17 txtar tests assert on individual ✓ lines). These features belong in
Phase 4 when `cfv .` becomes the unified command with its own distinct
output contract. Implementing them now required either (a) polluting the
reporter with mode flags, or (b) breaking check's output. Neither is
acceptable.

---

**Task 5**: YAML formatter (`pkg/formatter/yamlfmt/`)
- Library: `gopkg.in/yaml.v3` Node API (already a dep, zero new deps)
- Key: decode into `*yaml.Node`, walk tree normalizing indent/style, encode back
- Comment preservation: native via `HeadComment`/`LineComment`/`FootComment` fields
- Fixtures: ≥10 (indent width, block style, quote style, comment preservation, multi-doc)
- After YAML: register in `formatters.go` switch, add `yaml` and `yml` cases

**Task 5.5**: YAML formatter review fixes (8 items from 2026-06-30 deep review)

---

#### 5.5.1 — Fix multi-doc validation (silent data loss bug)

**Problem**: `yaml.Unmarshal` only validates the first document. If the second
document has a syntax error, the Decoder loop `break`s silently, and the
formatter returns only the valid documents — dropping the broken one without
any error.

Example: `"---\na: 1\n---\n{broken"` returns `"---\na: 1\n"` — silent data loss.

**Fix**:
- After the decode loop, capture the error from `dec.Decode`:
  ```go
  var decErr error
  for {
      var n yaml.Node
      if err := dec.Decode(&n); err != nil {
          decErr = err
          break
      }
      docs = append(docs, &n)
  }
  ```
- After the loop, if `decErr != io.EOF` (i.e., it was a real error, not
  end-of-stream), return it:
  ```go
  if decErr != nil && !errors.Is(decErr, io.EOF) {
      return nil, errors.New("yaml: " + decErr.Error())
  }
  ```
- The initial `yaml.Unmarshal` call stays — it catches duplicate keys that
  the Node decoder ignores. Add a comment explaining this.

**Files**: `pkg/formatter/yamlfmt/yaml.go`

**Tests**:
- Add case to `TestInvalidYAMLReturnsError`:
  `{"second doc broken", "---\na: 1\n---\n{broken"}`
- Add case: `{"third doc broken", "---\na: 1\n---\nb: 2\n---\n@ bad\n"}`
- Verify both return errors, not partial output.

---

#### 5.5.2 — Fix normalizeNode doc comment (lies about behavior)

**Problem**: Comment says "Removes TaggedStyle from scalars with standard
types" but the function body is a no-op stub.

**Fix**: Replace the doc comment with:
```go
// normalizeNode walks a yaml.Node tree. Currently a no-op — style
// normalisations (QuoteStyle conversion, flow→block expansion) will be
// added here as the corresponding Options fields are implemented.
```

**Files**: `pkg/formatter/yamlfmt/yaml.go`

**Tests**: None needed — comment-only change.

---

#### 5.5.3 — Fix Format doc comment (references non-existent QuoteStyle)

**Problem**: Line 49 says "Flow style: converted to block when
opts.QuoteStyle triggers it" — `Options` has no `QuoteStyle` field.

**Fix**: Replace that bullet with:
```
//   - Flow style: preserved (block conversion planned for a future option)
```

**Files**: `pkg/formatter/yamlfmt/yaml.go`

**Tests**: None needed — comment-only change.

---

#### 5.5.4 — Clarify why Unmarshal is needed alongside Decoder

**Problem**: The code validates with both `yaml.Unmarshal` and
`yaml.NewDecoder`. It's unclear why both exist.

**Fix**: Add a comment above the Unmarshal call:
```go
// Unmarshal into 'any' catches errors the Node decoder silently ignores,
// specifically duplicate mapping keys (yaml.v3's Decoder into *Node keeps
// both keys without error).
```

**Files**: `pkg/formatter/yamlfmt/yaml.go`

**Tests**: Add a test proving this behavior:
```go
func TestDuplicateKeysRejected(t *testing.T) {
    src := []byte("a: 1\na: 2\n")
    _, err := f.Format(src, defaultOpts)
    require.Error(t, err)
    require.Contains(t, err.Error(), "already defined")
}
```

---

#### 5.5.5 — Document !!merge tag behavior

**Problem**: The yaml.v3 encoder adds `!!merge` to `<<:` merge keys. This is
a semantic no-op but changes the serialized form. Users who diff after
formatting will see unexpected changes.

**Fix**: Add a note to the package doc comment:
```
// Known behaviors of the yaml.v3 encoder:
//   - Merge keys (<<: *anchor) gain an explicit !!merge tag in output.
//     This is semantically equivalent and idempotent.
```

Also add a brief comment in the anchors fixture test:
```go
// Note: yaml.v3 adds !!merge to << keys — this is expected and idempotent.
```

**Files**: `pkg/formatter/yamlfmt/yaml.go` (package doc),
`pkg/formatter/yamlfmt/yaml_test.go` (comment on anchors test, or in fixture)

**Tests**: The anchors fixture already verifies this (expected file has
`!!merge`). No additional test needed.

---

#### 5.5.6 — Document folded scalar joining behavior

**Problem**: The encoder canonicalises multi-line folded blocks (`>`) into
fewer lines. `"long\ndescription\nthat spans"` becomes a single long line.
This is spec-correct but changes visual layout.

**Fix**: Add to the package doc:
```
//   - Folded scalars (>) are canonicalised: the encoder joins lines per
//     the YAML folding rules, which may reduce line count.
```

Also add a comment in the multiline fixture expected file or test explaining
this is intentional.

**Files**: `pkg/formatter/yamlfmt/yaml.go` (package doc)

**Tests**: The multiline fixture already covers this. Add a brief comment in
`TestFixtures` isn't needed — the fixture name and expected file are
self-documenting.

---

#### 5.5.7 — Add FinalNewline=false test

**Problem**: No test verifies that `FinalNewline: false` strips the trailing
newline from YAML output.

**Fix**: Add:
```go
func TestFinalNewlineFalse(t *testing.T) {
    t.Parallel()
    src := []byte("a: 1\nb: 2\n")
    opts := defaultOpts
    opts.FinalNewline = false

    got, err := f.Format(src, opts)
    require.NoError(t, err)
    require.NotEqual(t, byte('\n'), got[len(got)-1],
        "expected no trailing newline, got: %q", got)
}
```

**Files**: `pkg/formatter/yamlfmt/yaml_test.go`

---

#### 5.5.8 — Fix idempotency test to use fixture-specific options

**Problem**: `TestIdempotency` formats all `.expected.yaml` files with
`defaultOpts` (2-space indent). For `indent_4.expected.yaml` (4-space),
the first format pass changes it to 2-space. The test still "passes"
(result is idempotent under defaults) but it doesn't verify the intended
contract: "the expected file is a fixed point under its own options."

**Fix**: Load the `.opts.json` sidecar for each fixture (if it exists) and
use those options for the idempotency check:

```go
func TestIdempotency(t *testing.T) {
    t.Parallel()
    expected, err := filepath.Glob("testdata/*.expected.yaml")
    require.NoError(t, err)
    require.NotEmpty(t, expected)

    for _, file := range expected {
        name := filepath.Base(file)
        t.Run(name, func(t *testing.T) {
            t.Parallel()
            src, err := os.ReadFile(file)
            require.NoError(t, err)

            // Use fixture-specific options when available.
            baseName := strings.TrimSuffix(name, ".expected.yaml")
            optsFile := "testdata/" + baseName + ".opts.json"
            opts := formatter.LoadFixtureOptions(optsFile, defaultOpts)

            first, err := f.Format(src, opts)
            require.NoError(t, err)
            second, err := f.Format(first, opts)
            require.NoError(t, err)

            require.Equal(t, string(first), string(second),
                "Format is not idempotent for %s", name)
        })
    }
}
```

This also validates that `indent_4.expected.yaml` is idempotent under
`IndentWidth: 4` — a stronger property.

**Files**: `pkg/formatter/yamlfmt/yaml_test.go`

---

### Execution order

```
5.5.1  (multi-doc validation)     — correctness bug, fix first
5.5.4  (duplicate keys comment)   — pairs with 5.5.1 (same area of code)
5.5.8  (idempotency test fix)     — test correctness, do before running tests
5.5.7  (FinalNewline test)        — one-liner
5.5.2  (normalizeNode comment)    — trivial
5.5.3  (Format comment)           — trivial
5.5.5  (!!merge docs)             — comment-only
5.5.6  (folded docs)              — comment-only
```

Estimated effort: Single session, ~30 minutes. All changes are in two files.

After all 8 fixes: run full pipeline, verify tests pass, proceed to Task 6.

**Task 6** (REVISED): Full vertical slice — config + CLI flags for JSON/YAML/HCL

Prove the entire stack works end-to-end on the 3 formatters we have before
adding more. This catches architectural issues early — better to fix the
Options struct, config resolution, or CLI ergonomics with 3 formatters than 8.

Also includes competitive-parity features pulled forward from 3.1:
- YAML sort-keys (walker in normalizeNode)
- quote-style option (double/single/preserve for YAML)
- --diff flag (unified diff output)
- .editorconfig integration (new resolution layer)

Hard constraint: the config file uses **cfv's vocabulary**, not library
internals. Users write `indent = 4`, not `SetIndent = 4`. The config keys
are the same for every format:

```toml
[format]
indent = 2              # applies to all formats
use-tabs = false
trailing-newline = true
sort-keys = false
line-ending = "lf"      # "lf" | "crlf"
quote-style = "preserve" # "double" | "single" | "preserve"

[format.json]
sort-keys = true        # override for JSON only

[format.yaml]
indent = 4              # override for YAML only
quote-style = "double"

[format.hcl]
# no options — canonical style, section accepted but empty
```

Resolution order: CLI flags > `[format.<type>]` > `[format]` > .editorconfig > hardcoded defaults.

Subtasks:
1. `[format]` section in `.cfv.toml` parser (global defaults)
2. `[format.json]`, `[format.yaml]` per-format overrides in parser
3. CLI flags: `--indent`, `--sort-keys`, `--line-ending` on `cfv format`
4. Resolution wiring: CLI flags > `[format.<type>]` > `[format]` > defaults
5. Schema validation of the `[format]` config (reject unknown keys)
6. Tests: txtar tests proving config file overrides, CLI flag overrides
7. Manual smoke test: demo repo with JSON + YAML + HCL, `.cfv.toml` with
   per-format overrides, verify output is correct

After this task: we have a **shippable product** for JSON/YAML/HCL with full
user configurability. Lessons learned here inform the remaining formatters.

**Task 7** (was Task 6): TOML, ENV, INI, XML, Properties formatters

Only start after Task 6 proves the config/CLI/Options architecture is solid.
Each formatter plugs into the same Options struct and config resolution — no
per-formatter config plumbing needed.

- TOML: `pelletier/go-toml/v2` unstable.Parser with KeepComments
- ENV: custom line-oriented formatter
- INI: `gopkg.in/ini.v1`
- XML: `go-xmlfmt/xmlfmt` (new dep)
- Properties: `magiconair/properties` (already a dep)

**Task 8**: Stress test + Opus review

### How to start the next session

```
cd /Users/se456c/src/github.com/boeing/config-file-validator
git checkout feat/3.0
go test ./... # verify all green
```

Then say "let's keep going" — next task is Task 8 (Final stress test + Opus review).

### Pipeline state

```
go vet ./...           ✅
gofmt -s -l -e .       ✅
golangci-lint run      ✅ 0 issues
go test ./...          ✅ all pass
coverage               ✅ 93.7%
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
