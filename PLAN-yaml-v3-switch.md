# Plan: Replace goccy/go-yaml with gopkg.in/yaml.v3

## Problem

goccy/go-yaml is too permissive as a validator. It accepts inputs that violate the YAML spec (`? >0`, `000:: '000'!`, mixed sequence/mapping at same level). These are rejected by yaml.v3, prettier, and every other spec-compliant parser. For a **validator project**, accepting invalid YAML is a correctness bug in our core product.

## Why yaml.v3

- Stricter: rejects invalid YAML that goccy permissively accepts
- Standard: the canonical Go YAML library, used by Kubernetes, Docker, and most of the Go ecosystem
- Already proven: our v2 codebase used yaml.v3 successfully — we're reverting to what worked
- Duplicate key detection: ✓ (was a concern, but yaml.v3 handles it)
- Multi-doc: ✓ via `yaml.NewDecoder` loop
- The original reasons for migrating TO goccy were AST/token access for formatting — we no longer use those (custom tokenizer replaces them entirely)

## Error Message Strategy

**goccy** provides: `[3:5] error message\n   1 | ...\n>  3 | ...\n       ^`
**yaml.v3** provides: `yaml: line 3: error message`

### Approach: Parse line from yaml.v3 errors (same as v2 did)

The v2 code already solved this with a regex: `yaml: line (\d+): (.*)`. This extracts the line number and wraps it in `ValidationError{Line: n, Err: msg}`.

**What we lose**: column number, source context display.
**What we keep**: line number, error message.
**Acceptable because**: 
- Users get the line number — they can find the problem
- The error message from yaml.v3 is descriptive ("mapping values are not allowed in this context")
- Column + context display is nice-to-have, not critical
- We can enhance errors later by reading the source line ourselves (deferred, not blocking)

### yaml.v3 error format variants

```
"yaml: line 3: mapping values are not allowed in this context"
"yaml: unmarshal errors:\n  line 2: mapping key \"a\" already defined at line 1"
"yaml: line 2: found character that cannot start any token"
"yaml: found unknown escape character"  (no line number!)
"yaml: line 2: found unexpected end of stream"
```

The regex `yaml: line (\d+): (.*)` handles most. For `yaml: unmarshal errors:` we need to parse the first sub-error. For errors without line numbers, we return the error as-is.

### Implementation

```go
var yamlLineRe = regexp.MustCompile(`(?:yaml: )?line (\d+): (.*)`)

func parseYAMLError(err error) error {
    msg := err.Error()
    // Try multi-error format first
    if strings.HasPrefix(msg, "yaml: unmarshal errors:") {
        // Extract first sub-error
        lines := strings.Split(msg, "\n")
        if len(lines) > 1 {
            msg = strings.TrimSpace(lines[1])
        }
    }
    if m := yamlLineRe.FindStringSubmatch(msg); m != nil {
        if line, convErr := strconv.Atoi(m[1]); convErr == nil {
            return &ValidationError{Err: errors.New(m[2]), Line: line}
        }
    }
    return err
}
```

## Detailed Changes

### 1. `pkg/validator/yaml.go`

**Current (goccy)**:
- Imports: `goyaml`, `ast`, `parser`
- `ValidateSyntax`: Decoder loop with `goyaml.NewDecoder`
- `MarshalToJSON`: `goyaml.Unmarshal`
- `ValidateSchema`: `goyaml.Unmarshal` + `parser.ParseBytes` + `walkYAMLNode` using goccy AST
- `buildYAMLPositionMap`: walks `ast.MappingNode`, `ast.SequenceNode`, etc.
- `parseValidationError`: parses goccy's `[line:col] msg\ncontext` format

**New (yaml.v3)**:
- Imports: `"gopkg.in/yaml.v3"`, drop `ast`, `parser`
- `ValidateSyntax`: Decoder loop with `yaml.NewDecoder` + `io.EOF` check (same pattern)
- `MarshalToJSON`: `yaml.Unmarshal` (drop-in)
- `ValidateSchema`: `yaml.Unmarshal` for JSON conversion + `yaml.Unmarshal` into `yaml.Node` for positions
- `buildYAMLPositionMap`: walk `yaml.Node` tree (Kind=MappingNode, Content pairs) — **copy from v2 code**
- `parseYAMLError`: regex parse `yaml: line N: message` — **copy from v2 code**

**Key difference in position map**: goccy uses `ast.MappingNode.Values[].Key.GetToken().Position.Line`. yaml.v3 uses `yaml.Node.Content[i].Line` (key nodes at even indices in mapping content). The v2 code already does this correctly.

### 2. `pkg/formatter/yamlfmt/yaml.go`

**Current**: 
```go
import goyaml "github.com/goccy/go-yaml"
...
var semanticCheck any
if err := goyaml.Unmarshal(src, &semanticCheck); err != nil {
    return nil, errors.New("yaml: " + err.Error())
}
```

**New**:
```go
import "gopkg.in/yaml.v3"
...
var semanticCheck any
if err := yaml.Unmarshal(src, &semanticCheck); err != nil {
    return nil, fmt.Errorf("yaml: %w", err)
}
```

Also: remove ALL dead goccy code (the old AST functions: `reindent`, `reindentByDepth`, `normalizeNode`, `sortMappingKeys`, `applyQuoteStyleToValue`, `hasFormattableRoot`, `resolveOptions` if only used by old path). Remove imports of `ast`, `parser`, `token`, `slices`, `strings` if unused.

### 3. `pkg/formatter/yamlfmt/yaml_test.go`

- Remove `"github.com/goccy/go-yaml/parser"` import
- Remove `parser.ParseBytes` usage in `FuzzYAMLFormatterWithOptions`
- Remove the `t.Skip` for goccy trailing-newline bug (yaml.v3 rejects those inputs before they reach the formatter)
- Use `yaml.Unmarshal` for output validation in fuzz test if needed (or just check idempotency)

### 4. `internal/generate/knownfiles/main.go`

```go
// Change:
goyaml "github.com/goccy/go-yaml"
goyaml.Unmarshal(data, &languages)

// To:
"gopkg.in/yaml.v3"
yaml.Unmarshal(data, &languages)
```

### 5. `go.mod`

- Remove: `github.com/goccy/go-yaml v1.19.2`
- Promote: `gopkg.in/yaml.v3 v3.0.1` from indirect to direct
- Run: `go mod tidy`

### 6. Test updates

- `pkg/validator/validator_test.go`: Error message assertions that check goccy-format strings (`[line:col] msg`) need updating to yaml.v3 format (`yaml: line N: msg`)
- `cmd/cfv/` txtar tests: Any that assert specific YAML error text
- `pkg/formatter/yamlfmt/yaml_test.go`: Remove goccy parser dependency, simplify fuzz tests

### 7. Cleanup

- Delete `goccy-bug-report.md` (no longer our concern)
- Remove all dead goccy AST code from `yaml.go` (reindent, normalizeNode, etc.)
- Update package doc comment in `yaml.go`

## Tasks (ordered)

- [ ] 1. `pkg/validator/yaml.go` — swap to yaml.v3, port v2 code for position map and error parsing
- [ ] 2. `pkg/formatter/yamlfmt/yaml.go` — swap Unmarshal, delete dead AST code
- [ ] 3. `pkg/formatter/yamlfmt/yaml_test.go` — remove goccy parser import, fix fuzz tests
- [ ] 4. `internal/generate/knownfiles/main.go` — swap import
- [ ] 5. `go mod tidy` — remove goccy dependency
- [ ] 6. Fix test failures — error message format changes
- [ ] 7. Run full pipeline — verify 0 lint, 0 vet, coverage ≥ 90%
- [ ] 8. Fuzz 45s on both formatter fuzz targets — expect cleaner results

## Risk

Low. We're reverting to proven code (v2 used yaml.v3). The API surface is nearly identical. The main work is test message updates and dead code removal.
