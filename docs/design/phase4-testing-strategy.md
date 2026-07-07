# Phase 4: Fix Engine — Testing Strategy

## The Core Challenge

A formatter has a simple correctness proof: `Format(Format(x)) == Format(x)`.

A fixer has a harder one: "the output is semantically correct AND the change was minimal AND the user's intent was preserved." You can't prove that with byte comparison alone.

## Testing Tiers

### Tier 1: Fixpoint Tests (automated, exhaustive)

Every fix rule gets a set of `{input, schema, expected_output}` triplets:

```
pkg/fixer/testdata/
  json_string_to_int/
    input.json          # {"port": "8080"}
    schema.json         # {"properties": {"port": {"type": "integer"}}}
    expected.json       # {"port": 8080}
  json_trailing_comma/
    input.json          # {"key": "value",}
    expected.json       # {"key": "value"}
  yaml_string_to_bool/
    input.yaml          # debug: "true"
    schema.json         # {"properties": {"debug": {"type": "boolean"}}}
    expected.yaml       # debug: true
```

The test:
1. Apply fixer to input with schema
2. Assert output == expected (byte-exact)
3. Assert output passes syntax validation
4. Assert output passes schema validation (THE KEY PROOF)
5. Assert fixer reports the correct fix metadata (rule ID, line, safety level)

**This is the "100% confidence" mechanism**: if the fixed output passes the schema that the original failed, the fix is provably correct.

### Tier 2: No-Regression Tests (automated)

For every file that ALREADY passes validation + schema:
- Apply fixer → output must be identical to input (no changes)
- Fixer must report zero fixes available

This proves the fixer doesn't touch files that are already correct.

Test corpus: the entire `test/` directory of this project (hundreds of valid config files across all formats).

### Tier 3: Blast Radius Tests (automated)

For every fix:
1. Parse input into a structured representation (JSON → map, YAML → nodes)
2. Apply fix
3. Parse output into same representation
4. Diff the two structures
5. Assert ONLY the target field changed — all other fields unchanged

This proves a fix to `port` doesn't accidentally modify `host`.

Implementation: for JSON/YAML/TOML, unmarshal before and after, deep-compare all paths except the fixed one.

### Tier 4: Idempotency Tests (automated)

```
Fix(Fix(input, schema), schema) == Fix(input, schema)
```

Applying the fixer twice must produce the same result. If it doesn't, the fixer is oscillating (fixing something, then "fixing" it back).

### Tier 5: Conflict Tests (automated, adversarial)

Create inputs with MULTIPLE schema violations that produce overlapping byte ranges:
- Two fixes target the same line
- A fix inside a value that another fix wants to delete
- A type coercion that changes value length (affecting byte offsets of subsequent fixes)

Assert:
- No panic
- Output is valid syntax
- Output passes schema (at least partially — some fixes may be dropped)
- Dropped fixes are reported (not silently lost)

### Tier 6: Fuzz Testing (automated, adversarial)

For each fix rule:
1. Generate random valid configs with the schema violation present
2. Apply fixer
3. Assert: no panic, output is valid syntax, output passes schema

For the fixer overall:
1. Feed arbitrary bytes + arbitrary schema
2. If both parse successfully, apply fixer
3. Assert: no panic, output is valid syntax if input was valid syntax

### Tier 7: Round-Trip Proof (automated, the ultimate test)

The strongest proof of correctness:

```
input → validator fails with schema errors
      → fixer produces fixed output + fix report
      → validator passes on fixed output
      → formatter produces formatted output
      → validator STILL passes on formatted output
```

If this chain holds for every fix rule × every format, the fix engine is correct.

This is the **integration test**: validate → fix → validate → format → validate. All three validation passes must succeed.

### Tier 8: Stress Testing (automated, adversarial)

Same philosophy as formatter stress testing:
- 60s fuzz per fix rule
- Real-world configs with schemas from SchemaStore
- Pathological inputs: deeply nested, very large, many violations
- Concurrent execution (50 goroutines applying fixes simultaneously)

### Anti-Patterns to Test For

| Anti-Pattern | Test |
|-------------|------|
| Fix introduces NEW schema error | Tier 1 (schema validation on output) |
| Fix corrupts unrelated values | Tier 3 (blast radius) |
| Fix changes valid file | Tier 2 (no-regression) |
| Fix oscillates (A→B→A) | Tier 4 (idempotency) |
| Overlapping fixes corrupt output | Tier 5 (conflict) |
| Fix panics on malformed input | Tier 6 (fuzz) |
| Fix breaks syntax | Tier 1 + Tier 7 (re-validate) |
| --fix in CI silently corrupts | Tier 7 (full pipeline proof) |

## Proof of "Works When Stressed"

The fix engine is PROVEN correct when ALL of the following hold simultaneously:

1. **Every fix rule has ≥10 fixpoint tests** that verify input→output→schema-passes
2. **Zero regressions on valid files** (entire test corpus unchanged after fix)
3. **Blast radius is minimal** (structure diff shows only target field changed)
4. **Idempotent** (Fix(Fix(x)) == Fix(x)) for all fixpoint tests
5. **Conflict-safe** (overlapping fixes don't corrupt)
6. **Fuzz-clean** (60s per rule, zero panics, all outputs valid)
7. **Round-trip proven** (validate→fix→validate→format→validate chain holds)

If ANY of these fail, the fix engine does not ship.

## Implementation Order (Test-First)

1. Write the fixpoint test harness FIRST (Tier 1 framework)
2. Implement fix rules one at a time, each with its test triplets
3. After each rule: run Tier 1-4 tests
4. After all safe rules done: run Tier 5-8 (stress)
5. Gate: all 7 criteria pass → merge

## Key Insight

The schema is our oracle. We don't need humans to verify fixes — we have a machine-checkable proof:

> "The file failed schema validation before. After the fix, it passes schema validation. The fix is correct."

This is stronger than what eslint or prettier can prove. They rely on rule authors being correct. We rely on the JSON Schema spec being correct — which it is, because it's a formal specification.
