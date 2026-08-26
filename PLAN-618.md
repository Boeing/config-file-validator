# PLAN-618: Remove Inline `$schema` Resolution from JSON/JSONC/TOML/TOON

## Session Status (2026-07-31)

**Phase 1-3 COMPLETE.** Committed as `4d1eb75` on `feat/3.0`. Pushed.

### Completed

- ✅ Task 1.1-1.4: Removed ValidateSchema from JSON, JSONC, TOML, TOON
- ✅ Task 1.5: Updated cli.go validateSchema (require-schema for JSONMarshaler types)
- ✅ Task 1.6: Removed dead code (buildJSONPositionMap)
- ✅ Task 2.1-2.7: Rewrote all 6 txtar tests for external-only resolution
- ✅ Task 2.8: Updated validator_test.go (removed old tests, updated MarshalToJSON tests)
- ✅ Task 2.9 + 7.1: Added 17 regression/stress tests
- ✅ Task 3.1: YAML MarshalToJSON handles .inf/.nan with clear error
- ✅ Pipeline: vet ✓, gofmt ✓, golangci-lint 0 ✓, build ✓, tests ✓, coverage 92.2%

### Remaining (future sessions)

- ⬜ Phase 4: Documentation updates (website docs)
- ⬜ Phase 5: Migration equivalence proof (50-repo validation)
- ⬜ Phase 6: Schema parity test suite
- ⬜ Phase 7: Stress testing (conversion fidelity edge cases)
- ⬜ CHANGELOG.md entry

## Decision

**Remove inline `$schema` resolution entirely from JSON, JSONC, TOML, and TOON.** The `$schema` property in a document is data, not a cfv directive. Schema resolution becomes external-only.

**Retained (no change):**
- YAML `# yaml-language-server: $schema=<url>` comment — out-of-band metadata, invisible to validation, no conflict possible
- XML `xsi:noNamespaceSchemaLocation` — W3C standard, part of XSD spec
- SARIF — built-in schema by version field, no user-facing declaration

**Rationale:** Inline `$schema` creates an inherent contradiction: the field is simultaneously a tooling directive (tells cfv what schema to use) AND a document property (that the schema may validate). All competing tools (ajv, v8r, VS Code JSON Language Service) resolve schemas externally and validate documents as-is. cfv was the only tool that stripped `$schema`, causing false rejections when schemas require it (#618) and masking true rejections when schemas forbid it.

---

## Root Cause Analysis

### The Bug (Issue #618)

`ValidateSchema` in JSON/JSONC/TOML/TOON:
1. Reads `$schema` URL from the document
2. **Deletes `$schema` from the document**
3. Validates the stripped document against the schema

If the schema has `"required": ["$schema"]` (like SchemaStore's catalog schema), validation fails with `"$schema is required"` — a false negative.

### Why It Existed

Original commit `ecf8007`: `// Remove $schema from document before validation — it's metadata, not content`

The author worried gojsonschema would confuse `$schema` in a data document with a meta-schema declaration. Testing proves this wrong — gojsonschema treats `$schema` in data documents as a regular property with no special behavior.

### Industry Validation

Tested three validators:

| Tool | Strips `$schema`? | Reads `$schema` for resolution? |
|------|:-----------------:|:-------------------------------:|
| ajv | No | No (explicit --schema flag) |
| v8r | No | No (catalog/filename matching) |
| VS Code JSON LS | No | No (workspace settings, catalog) |
| cfv (current) | **Yes** | **Yes** |

All three treat `$schema` in documents as data. None use it for schema resolution.

---

## Scope of Change

### Code Paths Affected

| File | Method | Line | Current Behavior | New Behavior |
|------|--------|------|-----------------|--------------|
| `pkg/validator/json.go` | `ValidateSchema` | 57-91 | Read+delete `$schema`, validate stripped doc | **Remove method entirely** |
| `pkg/validator/json.go` | `MarshalToJSON` | 45-55 | Delete `$schema`, marshal | Marshal without deletion |
| `pkg/validator/jsonc.go` | `ValidateSchema` | 49-87 | Read+delete `$schema`, validate stripped doc | **Remove method entirely** |
| `pkg/validator/jsonc.go` | `MarshalToJSON` | 33-47 | Delete `$schema`, marshal | Marshal without deletion |
| `pkg/validator/toml.go` | `ValidateSchema` | 42-68 | Read+delete `$schema`, validate stripped doc | **Remove method entirely** |
| `pkg/validator/toml.go` | `MarshalToJSON` | 33-40 | Delete `$schema`, marshal | Marshal without deletion |
| `pkg/validator/toon.go` | `ValidateSchema` | 44-75 | Read+delete `$schema`, validate stripped doc | **Remove method entirely** |
| `pkg/validator/toon.go` | `MarshalToJSON` | 32-42 | Delete `$schema`, marshal | Marshal without deletion |
| `pkg/validator/schema.go` | `resolveSchemaURL` | 58-72 | Resolves relative/absolute/URL schemas | Keep — used by YAML |
| `pkg/cli/cli.go` | `validateSchema` | 329-366 | Inline > schema-map > schemastore | **schema-map > schemastore** |

### Interfaces

- `SchemaValidator` interface — **kept**. Still used by YAML (comment), XML (xsi), SARIF (built-in).
- `JSONMarshaler` interface — **kept**. Still used by the external schema path (schema-map/schemastore).
- JSON/JSONC/TOML/TOON no longer implement `SchemaValidator`. They still implement `JSONMarshaler`.

### Priority Change

Before:
```
1. Document inline $schema (JSON/JSONC/TOML/TOON)
2. --schema-map
3. --schemastore
```

After:
```
1. Document schema declaration (YAML comment, XML xsi, SARIF version only)
2. --schema-map
3. --schemastore
```

For JSON/JSONC/TOML/TOON, the `SchemaValidator` type assertion fails → falls through directly to schema-map/schemastore.

---

## Breaking Change Assessment

### Who Uses Inline `$schema` Today?

From 50 realworld repos (2041 files with `$schema`):

- **~2726 are meta-schema declarations** (`http://json-schema.org/draft-07/schema#`) — these are schema FILES, not config files being validated. Unaffected.
- **~300 are actual config files** using inline `$schema` for validation:
  - **SchemaStore-matched** (tsconfig, renovate, turbo, project.json, nx) — these continue working via SchemaStore filename matching. **No action needed.**
  - **Non-SchemaStore** (biome, shadcn, tauri, pulumi, grafana) — these repos use relative paths or custom URLs. **Need `--schema-map` or `.cfv.toml`.**

### Migration Path

| User Scenario | Impact | Migration |
|---------------|--------|-----------|
| Files matched by SchemaStore (package.json, tsconfig, etc.) | **None** — SchemaStore resolves by filename | No action |
| Files with `$schema` pointing to SchemaStore URLs | **None** — SchemaStore matches by filename regardless | No action |
| Files with `$schema` pointing to custom/relative schemas | **Broken** — no schema resolution occurs | Add to `.cfv.toml` `[schema-map]` |
| `--require-schema` with inline `$schema` files | **Changed** — file fails unless schema-map/schemastore matches | Add external mapping |
| `$schema` in document + `additionalProperties: false` schema | **Now correctly reported as invalid** | Remove `$schema` from document or add to schema's properties |

### Test Fixtures Affected

| File | Current | Change |
|------|---------|--------|
| `cmd/cfv/testdata/json_schema.txtar` | All tests use inline `$schema` | Rewrite to use `--schema-map` |
| `cmd/cfv/testdata/toml_schema.txtar` | Uses inline `$schema` | Rewrite to use `--schema-map` |
| `cmd/cfv/testdata/toon_schema.txtar` | Uses inline `$schema` | Rewrite to use `--schema-map` |
| `cmd/cfv/testdata/schema_map.txtar` | Tests "$schema takes priority" | Remove priority test |
| `cmd/cfv/testdata/schemastore.txtar` | Tests "$schema takes priority" | Remove priority test |
| `cmd/cfv/testdata/no_schema.txtar` | Tests `--no-schema` skips inline | Update — `$schema` in doc is now inert |
| `cmd/cfv/testdata/stress_v2_features.txtar` | Stdin with inline `$schema` | Rewrite to use `--schema-map` where possible |
| `cmd/cfv/testdata/yaml_schema.txtar` | Uses YAML comment (retained) | **No change** |
| `pkg/validator/validator_test.go` | Unit tests for ValidateSchema, MarshalToJSON | Remove ValidateSchema tests for JSON/JSONC/TOML/TOON; update MarshalToJSON tests |

### Documentation Affected

| File | Change |
|------|--------|
| `website/docs/guides/schema-validation.md` | Remove "Declaring a schema in your files" for JSON/TOML/TOON. Update priority. |
| `website/docs/quick-start.md` | Remove mention of `$schema` auto-detection |
| `website/docs/guides/stdin.md` | Update — stdin schema requires explicit flag, not inline |
| `CHANGELOG.md` | Breaking change entry |

---

## Implementation Tasks

### Phase 1: Remove Inline `$schema` Resolution

#### Task 1.1: Remove `ValidateSchema` from JSON validator

**File:** `pkg/validator/json.go`

Delete the `ValidateSchema` method (lines 57-91) entirely. JSON no longer implements `SchemaValidator`.

Keep `MarshalToJSON` but remove the `delete(doc, "$schema")`:

```go
func (JSONValidator) MarshalToJSON(b []byte) ([]byte, error) {
	var raw any
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	return json.Marshal(raw)
}
```

#### Task 1.2: Remove `ValidateSchema` from JSONC validator

**File:** `pkg/validator/jsonc.go`

Delete the `ValidateSchema` method (lines 49-87) entirely.

Update `MarshalToJSON` — remove `delete(doc, "$schema")`:

```go
func (JSONCValidator) MarshalToJSON(b []byte) ([]byte, error) {
	standardized, err := hujson.Standardize(bytes.Clone(b))
	if err != nil {
		return nil, err
	}
	return standardized, nil
}
```

Note: `hujson.Standardize` produces valid JSON. No need to unmarshal/re-marshal.

#### Task 1.3: Remove `ValidateSchema` from TOML validator

**File:** `pkg/validator/toml.go`

Delete the `ValidateSchema` method (lines 42-68) entirely.

Update `MarshalToJSON` — remove `delete(doc, "$schema")`:

```go
func (TomlValidator) MarshalToJSON(b []byte) ([]byte, error) {
	var doc map[string]any
	if err := toml.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	return json.Marshal(doc)
}
```

#### Task 1.4: Remove `ValidateSchema` from TOON validator

**File:** `pkg/validator/toon.go`

Delete the `ValidateSchema` method (lines 44-75) entirely.

Update `MarshalToJSON` — remove `delete(doc, "$schema")`:

```go
func (ToonValidator) MarshalToJSON(b []byte) ([]byte, error) {
	raw, err := toon.Decode(b)
	if err != nil {
		return nil, err
	}
	return json.Marshal(raw)
}
```

#### Task 1.5: Simplify `cli.go` validateSchema

**File:** `pkg/cli/cli.go`

The flow simplifies. For JSON/JSONC/TOML/TOON, the `SchemaValidator` type assertion now fails, so it skips directly to schema-map/schemastore:

```go
func (c *CLI) validateSchema(v validator.Validator, content []byte, filePath string) (bool, []string, error) {
	if c.noSchema {
		return true, nil, nil
	}

	// YAML, XML, SARIF — formats with spec-defined schema declarations
	sv, hasSV := v.(validator.SchemaValidator)
	if hasSV {
		valid, err := sv.ValidateSchema(content, filePath)
		if !errors.Is(err, validator.ErrNoSchema) {
			return valid, nil, err
		}
	}

	// External schema resolution: schema-map > schemastore
	if schemaPath, ok := c.lookupSchemaMap(filePath); ok {
		valid, skipped, err := validateWithExternal(v, content, schemaPath)
		if skipped {
			if c.requireSchema {
				return false, nil, &validator.SchemaErrors{
					Items: []string{schemaMapUnsupportedError(schemaPath)},
				}
			}
			return valid, []string{schemaMapUnsupportedWarning(schemaPath)}, nil
		}
		return valid, nil, err
	}

	if c.schemaStore != nil {
		if schemaPath, ok := c.schemaStore.Resolve(filePath); ok {
			valid, _, err := validateWithExternal(v, content, schemaPath)
			return valid, nil, err
		}
	}

	// requireSchema: fail if format supports schema but none was found
	// hasSV covers YAML/XML/SARIF; JSONMarshaler covers JSON/JSONC/TOML/TOON
	if c.requireSchema {
		if hasSV {
			return false, nil, validator.ErrNoSchema
		}
		if _, hasJM := v.(validator.JSONMarshaler); hasJM {
			return false, nil, validator.ErrNoSchema
		}
	}
	return true, nil, nil
}
```

Note the `--require-schema` change: previously it only triggered for `SchemaValidator` implementors. Now it also triggers for `JSONMarshaler` implementors (JSON/JSONC/TOML/TOON) since they CAN be schema-validated via external schemas but none was found.

#### Task 1.6: Clean up dead code

**Files:**
- `pkg/validator/schema.go` — `resolveSchemaURL` is still used by YAML. Keep.
- `pkg/validator/json.go` — `buildJSONPositionMap` is no longer called from `ValidateSchema`. Check if it's used elsewhere.

Check: `buildJSONPositionMap` — used from JSON's `ValidateSchema` only? If yes, consider whether it should be used in the external path too (for better error messages from schema-map/schemastore). Decision: defer to a later enhancement. For now, keep the function (it's used by YAML via the position map pattern).

Actually — `buildJSONPositionMap` is only called from JSON's `ValidateSchema`. After removal, it becomes dead code UNLESS we wire it into `validateWithExternal`. We should wire it in for JSON files validated via external schemas. This gives position-aware error messages for all JSON schema validation.

**Change to `validateWithExternal`:** When the validator is JSON and we call `JSONSchemaValidate`, optionally build the position map:

```go
func validateWithExternal(v validator.Validator, content []byte, schemaPath string) (valid bool, skipped bool, err error) {
	if _, ok := v.(validator.XMLSchemaValidator); ok {
		absSchema, err := filepath.Abs(schemaPath)
		if err != nil {
			return false, false, fmt.Errorf("resolving schema path: %w", err)
		}
		valid, err := validator.ValidateXSD(content, absSchema)
		return valid, false, err
	}

	jm, ok := v.(validator.JSONMarshaler)
	if !ok {
		return true, true, nil
	}

	schemaURL, err := toSchemaURL(schemaPath)
	if err != nil {
		return false, false, err
	}

	docJSON, err := jm.MarshalToJSON(content)
	if err != nil {
		return false, false, err
	}

	posMap := validator.BuildPositionMap(v, content)
	valid, err = validator.JSONSchemaValidateWithPositions(schemaURL, docJSON, posMap)
	return valid, false, err
}
```

This requires exporting a `BuildPositionMap` helper or using a new interface. Alternatively, just use `JSONSchemaValidate` (no positions) for the external path and leave position-aware errors as a future enhancement. **Decision: keep it simple for now — use `JSONSchemaValidate` without positions for external path. Position support for external schemas is a separate issue.**

### Phase 2: Update Tests

#### Task 2.1: Rewrite `json_schema.txtar`

All tests that used inline `$schema` now use `--schema-map`. Add a test proving `$schema` in the document is treated as data (validated like any other property).

#### Task 2.2: Rewrite `toml_schema.txtar`

Same pattern — `--schema-map` instead of inline.

#### Task 2.3: Rewrite `toon_schema.txtar`

Same pattern — `--schema-map` instead of inline.

#### Task 2.4: Update `schema_map.txtar`

Remove the "Document `$schema` takes priority" test. Add a test showing `$schema` in document is validated as data when schema has `additionalProperties: false`.

#### Task 2.5: Update `schemastore.txtar`

Remove "PART 7: Document `$schema` takes priority over --schemastore" test.
Remove `valid/with_own_schema.json` and `valid/own_schema.json` test files.

#### Task 2.6: Update `no_schema.txtar`

The `--no-schema` flag still disables all schema validation (via schema-map/schemastore). Update tests: `$schema` in documents is inert — the test should show it has no effect with or without `--no-schema`.

#### Task 2.7: Update `stress_v2_features.txtar`

Stdin tests with inline `$schema` no longer get schema validation. Either:
- Remove those tests (stdin + schema is covered by `--schema-map` which doesn't apply to stdin)
- Change to show that `$schema` in stdin content is just data (passes syntax)

Note: stdin docs cannot use `--schema-map` (no filename to match). stdin + schema requires a new mechanism OR is simply not supported. Document this as a known limitation.

#### Task 2.8: Update `pkg/validator/validator_test.go`

- Delete all `Test_JSON*ValidateSchema*` tests (no longer has ValidateSchema)
- Delete all `Test_Toml*ValidateSchema*` tests
- Delete all `Test_Toon*ValidateSchema*` tests
- Delete `Test_JSONC*ValidateSchema*` tests
- Keep `Test_YAML*ValidateSchema*` tests (comment-based, retained)
- Keep `Test_XML*ValidateSchema*` tests (xsi-based, retained)
- Keep `Test_Sarif*ValidateSchema*` tests
- Update `Test_JSONMarshalToJSON` — verify `$schema` is preserved in output
- Update `Test_JSONCMarshalToJSON` — verify `$schema` is preserved in output
- Update `Test_TomlMarshalToJSON` — verify `$schema` is preserved in output  
- Update `Test_ToonMarshalToJSON` — verify `$schema` is preserved in output
- Delete `resolveSchemaURL` tests that are only reachable from removed code (keep if YAML still uses them)

#### Task 2.9: Add new regression tests for #618

Add tests proving the fix works:
1. JSON document with `$schema` + schema that requires `$schema` → validated via schema-map → PASSES
2. JSON document with `$schema` + schema with `additionalProperties: false` not listing `$schema` → validated via schema-map → correctly FAILS
3. TOML/TOON same patterns

### Phase 3: Fix Conversion Fidelity Issues

#### Task 3.1: Handle YAML `.inf`/`.nan` in MarshalToJSON

**File:** `pkg/validator/yaml.go`

`json.Marshal` crashes on `+Inf`, `-Inf`, `NaN` (Go floats that JSON can't represent). The current `MarshalToJSON` has no guard.

**Fix:** Walk the unmarshaled document BEFORE marshaling. If any `±Inf` or `NaN` value is found, return an explicit error. Do NOT silently replace values — that would be the same category of bug as stripping `$schema` (modifying data before validation, causing false results).

```go
func (YAMLValidator) MarshalToJSON(b []byte) ([]byte, error) {
	var doc any
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	if err := checkJSONRepresentable(doc); err != nil {
		return nil, err
	}
	return json.Marshal(doc)
}

// checkJSONRepresentable walks a YAML-decoded value tree and returns an error
// if any float is ±Inf or NaN — values JSON cannot represent. Schema validation
// cannot proceed on documents containing these values.
func checkJSONRepresentable(v any) error {
	switch val := v.(type) {
	case map[string]any:
		for k, item := range val {
			if err := checkJSONRepresentable(item); err != nil {
				return fmt.Errorf("key %q: %w", k, err)
			}
		}
	case []any:
		for i, item := range val {
			if err := checkJSONRepresentable(item); err != nil {
				return fmt.Errorf("index %d: %w", i, err)
			}
		}
	case float64:
		if math.IsInf(val, 1) {
			return errors.New("value .inf cannot be represented in JSON; schema validation cannot be performed")
		}
		if math.IsInf(val, -1) {
			return errors.New("value -.inf cannot be represented in JSON; schema validation cannot be performed")
		}
		if math.IsNaN(val) {
			return errors.New("value .nan cannot be represented in JSON; schema validation cannot be performed")
		}
	}
	return nil
}
```

The error message tells the user exactly what's wrong and why. No silent data manipulation.

#### Task 3.2: Document large int64 precision loss

**No code fix.** JSON fundamentally uses float64 for numbers. Int64 values > 2^53 lose precision. This affects schema validation for `maximum`/`minimum` constraints on large integers. This matches the behavior of every JSON Schema validator (including ajv) since JSON itself has this limitation. Document as a known limitation.

### Phase 4: Update Documentation

#### Task 4.1: Rewrite schema-validation.md

- Remove "Declaring a schema in your files" sections for JSON, TOML, TOON
- Keep YAML comment section, XML xsi section, SARIF section
- Update priority order (remove inline from list)
- Add migration section: "Migrating from inline `$schema`"
- Emphasize `.cfv.toml` `[schema-map]` as the primary mechanism

#### Task 4.2: Update quick-start.md

Remove: "Files that declare a `$schema` (JSON, TOML) are validated against their schema automatically."
Replace with: "Use `--schemastore` for automatic schema validation, or `--schema-map` for custom schemas."

#### Task 4.3: Update stdin.md

Update: stdin schema validation requires `--schema-map` with a pattern that matches `-` (stdin), or simply document that stdin content is syntax-validated only (no schema).

Actually check: does `lookupSchemaMap` match `stdin` or `-`? Need to verify.

#### Task 4.4: CHANGELOG.md

Under `[Unreleased]` → `Changed` (breaking):
```
- Schema resolution no longer reads `$schema` from JSON, JSONC, TOML, or TOON documents. Use `--schema-map`, `.cfv.toml` `[schema-map]`, or `--schemastore` for schema validation. YAML comment (`# yaml-language-server: $schema=...`) and XML `xsi:noNamespaceSchemaLocation` are unaffected. (#618)
```

Under `[Unreleased]` → `Fixed`:
```
- Schema validation no longer silently strips `$schema` from documents before validation, fixing false rejections when schemas require `$schema` as a property (#618)
- YAML files containing `.inf` or `.nan` now report a clear error during schema validation instead of crashing
```

### Phase 5: Migration Equivalence Proof

**Goal:** Prove with 100% certainty that every file that validated successfully via inline `$schema` continues to validate identically via `--schema-map`.

#### Task 5.1: Build migration equivalence harness

Located at `~/.cfv-schema-parity/migration-proof/`. Self-contained script.

**Methodology:**

1. **Collect:** Find every file in the test fixtures and realworld repos that uses inline `$schema` for validation (not meta-schema declarations like `json-schema.org/draft-*`).
2. **Extract:** For each file, parse out the `$schema` URL/path it declares.
3. **Baseline (current cfv):** Run current cfv (pre-change) against each file. Record: pass/fail, error messages.
4. **Migration (new cfv):** Run new cfv with `--schema-map=<file>:<extracted-schema>` against the same file. Record: pass/fail, error messages.
5. **Compare:** For every file:
   - Same pass/fail verdict? If not → **MIGRATION FAILURE**, stop and investigate.
   - If both fail, same error category? (schema error vs. syntax error)
   - Error message differences are expected (position info may differ since `$schema` is now validated as data) — but the verdict must match.

**Key distinction:** Files where `$schema` is stripped AND the schema has `additionalProperties: false` without `$schema` in properties will CORRECTLY change from pass → fail. These are NOT migration failures — they are bug fixes. The harness must categorize these separately:

- **Category A: Identical verdict** — migration works
- **Category B: Pass → Fail due to `additionalProperties`** — correct behavior change (the document was always invalid per spec, cfv was hiding it)
- **Category C: Unexpected verdict change** — real migration failure, requires investigation

#### Task 5.2: Run against test fixtures

All files in:
- `cmd/cfv/testdata/json_schema.txtar` (inline `$schema` tests)
- `cmd/cfv/testdata/toml_schema.txtar`
- `cmd/cfv/testdata/toon_schema.txtar`
- `cmd/cfv/testdata/stress_v2_features.txtar`
- `cmd/cfv/testdata/schemastore.txtar` (the `with_own_schema.json` file)
- `cmd/cfv/testdata/schema_map.txtar` (the `with_schema.json` file)

**Expected:** Category A for all files where the schema allows `$schema` as a property (or doesn't use `additionalProperties: false`). Category B for `strict_schema.json` tests (known, intentional).

#### Task 5.3: Run against realworld repos

For the ~300 config files with inline `$schema` pointing to actual validation schemas:

1. Fetch each schema (from URL or resolve relative path)
2. Run both old and new cfv
3. Categorize results

**Expected:** Category A for vast majority. Category B for files using schemas with `additionalProperties: false` that don't list `$schema` (like nodemon.json if it had inline `$schema`).

**Any Category C result is a ship-blocker.** Zero tolerance.

#### Task 5.4: Run against SchemaStore overlap

For files that have BOTH inline `$schema` AND a SchemaStore filename match:
1. Current cfv: inline `$schema` wins (priority 1)
2. New cfv: SchemaStore matches by filename (priority 2, now effectively priority 1)
3. Verify: same schema is applied in both cases (the inline URL should match the SchemaStore catalog entry)

If a file's inline `$schema` points to a DIFFERENT schema than SchemaStore would resolve → that's a real behavioral change. Identify all such cases and document them.

#### Task 5.5: Produce migration report

Output format:
```
MIGRATION EQUIVALENCE REPORT
=============================
Total files tested: N
Category A (identical): N (X%)
Category B (correct fix): N (X%)  
Category C (FAILURE): N (X%)

Category B details:
  - file: reason (additionalProperties:false, $schema not in properties)

Category C details:
  - file: old=PASS new=FAIL reason=???
```

**Pass criteria:** Category C = 0.

### Phase 6: Schema Parity Test Suite (General)

#### Design

Located at `~/.cfv-schema-parity/`. Modeled after `~/.cfv-parity/` (formatting).

**Reference tools:**
- ajv-cli (JSON Schema validation, all drafts)
- v8r (SchemaStore-based validation)

**What it tests:**
1. **Agreement:** Does cfv produce the same pass/fail result as ajv for the same document+schema pair?
2. **SchemaStore coverage:** For files matched by SchemaStore, does cfv agree with v8r?
3. **Format conversion fidelity:** YAML/TOML validated against a schema — does cfv agree with ajv on the JSON-converted form?

**Test matrix:**

| Format | Resolution | Reference | Test |
|--------|-----------|-----------|------|
| JSON | schema-map (local schema) | ajv | Byte-for-byte agreement on pass/fail |
| JSON | SchemaStore | v8r | Agreement on pass/fail |
| JSONC | schema-map | ajv (on standardized JSON) | Agreement |
| YAML | schema-map | ajv (on YAML→JSON conversion) | Agreement |
| YAML | SchemaStore | v8r | Agreement |
| TOML | schema-map | ajv (on TOML→JSON conversion) | Agreement |
| TOON | schema-map | ajv (on TOON→JSON conversion) | Agreement |

**Conversion fidelity sub-suite:**

For each YAML/TOML/TOON file:
1. Convert to JSON via cfv's `MarshalToJSON`
2. Convert to JSON via reference method (yq for YAML, taplo for TOML)
3. Validate both JSON forms against the same schema with ajv
4. Compare results — any disagreement is a fidelity bug

**Test data sources:**
- Same 12 repos as formatting parity suite
- Additional repos with heavy schema usage: SchemaStore/schemastore itself, biome, tauri, pulumi
- Synthetic edge cases: YAML with .inf/.nan, TOML with dates, large integers

#### Implementation

```
~/.cfv-schema-parity/
├── run                      # Main runner script
├── run.mjs                  # Node.js implementation
├── package.json             # ajv-cli, v8r dependencies
├── repos/                   # Cloned test repos
├── schemas/                 # Local schema copies for offline testing
├── results/                 # Per-run results
├── state.json               # Persistent state across runs
├── fidelity/                # Conversion fidelity test cases
│   ├── yaml-edge-cases.yml
│   ├── toml-edge-cases.toml
│   └── toon-edge-cases.toon
└── README.md
```

**Runner logic:**
1. Build cfv from current branch
2. For each test file + schema pair:
   a. Run `cfv check --schema-map=<file>:<schema> <file>` → record pass/fail
   b. Run `ajv validate -s <schema> -d <converted-json>` → record pass/fail
   c. Compare
3. For SchemaStore files:
   a. Run `cfv check --schemastore <file>` → record pass/fail
   b. Run `v8r <file>` → record pass/fail
   c. Compare
4. Report disagreements with details (file, schema, cfv result, reference result)
5. Save to state.json

**Tracked categories:**
- `agree-pass` — both say valid
- `agree-fail` — both say invalid
- `cfv-false-positive` — cfv says valid, reference says invalid
- `cfv-false-negative` — cfv says invalid, reference says valid
- `cfv-error` — cfv crashes/errors, reference completes
- `ref-error` — reference crashes, cfv completes

**Success criteria:** 0 false positives, 0 false negatives across all tested files.

### Phase 7: Stress Testing

#### Task 7.1: Schema validation stress tests

Add to `cmd/cfv/testdata/` or a dedicated stress txtar:

1. Document with `$schema` that is NOT used for resolution — validated via schema-map, `$schema` treated as data
2. Document with `$schema` validated against schema with `additionalProperties: false` — correctly fails
3. Document with `$schema` validated against schema that REQUIRES `$schema` — correctly passes
4. YAML with `.inf` + schema validation — no crash
5. YAML with `.nan` + schema validation — no crash
6. TOML with datetime fields + schema requiring `"type": "string"` — passes
7. Large integer in YAML/TOML — schema with `maximum` constraint at int53 boundary
8. Deeply nested document — schema validation works on nested `$schema` keys without stripping
9. Document with `$schema: null` or `$schema: 123` — treated as data, schema validates (or rejects) based on schema rules
10. Multiple files in one run — some matched by SchemaStore, some by schema-map, some by neither
11. `--require-schema` with no schema source available — fails for JSON/JSONC/TOML/TOON
12. `--require-schema` with SchemaStore available — passes for matched files, fails for unmatched

#### Task 7.2: Conversion fidelity stress

Edge cases for YAML→JSON:
1. Anchors with override (`<<: *defaults` + overriding keys)
2. Binary data (`!!binary`)
3. Multi-document YAML (only first document used for schema?)
4. Empty document
5. Null document (`---\n~`)
6. Sequence at root (not an object)
7. Complex keys (`{a: 1, b: 2}: value`)

Edge cases for TOML→JSON:
1. Dotted keys (`a.b.c = 1` → `{"a":{"b":{"c":1}}}`)
2. Mixed array of tables and inline tables
3. Unicode keys
4. Empty tables
5. Integer boundaries (int64 max, int64 min)

### Phase 8: Pipeline and Coverage

#### Task 8.1: Run full pipeline

```bash
go vet ./...
test -z "$(gofmt -s -l -e .)"
golangci-lint run ./...
go build -o /dev/null cmd/validator/validator.go
go test -cover -coverprofile coverage.out ./...
go tool cover -func coverage.out | grep total
```

Must pass with ≥ 90% coverage.

#### Task 8.2: Coverage analysis

Removing `ValidateSchema` from 4 validators removes both production code AND tests. Verify coverage doesn't drop below 90%. If it does, the new schema-map-based tests must cover the remaining external validation paths adequately.

---

## `--require-schema` Semantics After Change

**Before:** "Fail if the document supports schema validation and doesn't declare a schema" — meaning no inline `$schema`.

**After:** "Fail if the document's format supports schema validation and no schema was found from any source (schema-map or schemastore)."

This means `--require-schema` without `--schemastore` or `--schema-map` will fail ALL JSON/JSONC/TOML/TOON files (since there's no way to find a schema). That's the correct behavior — it forces users to configure schema sources.

---

## Stdin Interaction

**Before:** stdin with inline `$schema` worked — the validator read the URL from the content.

**After:** stdin has no filename → schema-map can't match, SchemaStore can't match. **stdin loses schema validation entirely** unless we add a mechanism.

**Options:**
1. Accept the limitation — stdin is syntax-only (document clearly)
2. Add `--schema <url>` flag for explicit one-off schema (like ajv's `-s` flag)
3. Accept `--schema-map=-:<schema>` where `-` means stdin

**Decision:** Option 2 — add `--schema <url-or-path>` flag. This is the exact model ajv uses. It applies to ALL files in the invocation (like `--schema-map` but without a pattern). Useful for stdin and for "validate this one file against this specific schema" workflows.

This is a new feature, not blocking for the #618 fix. Can be implemented as a follow-up. For v3 release: document that stdin schema requires `--schema`.

---

## Verification Criteria

- [ ] **Migration equivalence proof: Category C = 0** (zero unexpected verdict changes)
- [ ] Migration equivalence Category B documented and justified (each is a correct behavior fix)
- [ ] Reporter's exact repro passes: `echo '{"$schema":"...schema-catalog.json","version":1,"schemas":[]}' | cfv check --schema-map=-:schema-catalog-schema.json --file-types=json -`
- [ ] `$schema` in documents is validated as data (not stripped)
- [ ] Schema-map with `additionalProperties: false` + `$schema` in doc → correctly rejects
- [ ] Schema-map with `required: ["$schema"]` + `$schema` in doc → correctly passes
- [ ] YAML comment-based schema still works
- [ ] XML xsi-based schema still works
- [ ] SARIF built-in schema still works
- [ ] SchemaStore resolution works for all formats
- [ ] `--require-schema` fails JSON/JSONC/TOML/TOON when no external schema source configured
- [ ] YAML `.inf`/`.nan` produces a clear error during schema validation (not a crash, not silent corruption)
- [ ] MarshalToJSON preserves `$schema` in output for all formats
- [ ] All existing formatting tests still pass (no regression)
- [ ] Pipeline passes, coverage ≥ 90%
- [ ] golangci-lint clean
- [ ] Schema parity suite: 0 false positives, 0 false negatives
- [ ] Conversion fidelity: cfv agrees with ajv on YAML→JSON and TOML→JSON validated documents

---

## Commit Plan

1. `refactor(validator): remove inline $schema resolution from JSON/JSONC/TOML/TOON` — code changes only
2. `fix(validator): preserve $schema in MarshalToJSON output` — removes delete calls
3. `fix(yaml): handle .inf/.nan in MarshalToJSON` — fidelity fix
4. `test(schema): rewrite schema tests for external-only resolution` — all test updates
5. `feat(cli): update --require-schema for external-only model` — semantics change
6. `docs: update schema validation guide for v3 external-only model` — docs
7. `test(parity): add schema parity test suite` — parity infrastructure

Each commit independently passes the pipeline. Atomic, reviewable, bisectable.
