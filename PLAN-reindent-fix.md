# Plan: Fix YAML Reindent for Continuation Values

## The Problem

`computeDepths` assigns a structural depth to EVERY `TokIndent` token equally. It cannot distinguish between:

1. **Structural indent** — a nested mapping key or sequence entry that should be independently renormalized
2. **Continuation indent** — a plain scalar value on a continuation line that should move WITH its parent key

Example:
```yaml
key:
  continuation value    ← this is NOT a nested key, it's the VALUE of "key"
nested_key:
  actual_child: here    ← this IS a nested key
```

After reindent from 2→4 spaces, the correct output is:
```yaml
key:
    continuation value    ← indent shifted by same delta as parent
nested_key:
    actual_child: here    ← independently renormalized
```

But our current code treats both the same — renormalize to depth×width.

## How Other Tools Handle This

### prettier (JavaScript)

prettier does NOT do token-level reindentation. It uses a **tree-based document IR**:
- Parses YAML into a full AST (via yaml-unist-parser) that understands the STRUCTURE
- Walks the AST to produce a document IR (`hardline`, `indent`, `align`, etc.)
- The IR engine handles indentation deterministically
- For plain scalars: it JOINS continuation lines onto the key line (`key:\n  value` → `key: value`)
- Multi-line plain scalars get reflowed based on `proseWrap` setting
- Block scalars stay multi-line (handled by `printBlock` with explicit `alignWithSpaces`)

Key insight: **prettier never manipulates indent tokens directly.** It reconstructs the entire output from a structural understanding of the document. There is no "shift this indent by delta" — there is only "this node is at depth 2, indent it accordingly."

### Google yamlfmt (Go)

yamlfmt uses a **decode/re-encode approach**:
- Decodes YAML into `yaml.Node` tree
- The encoder (`yaml.Encoder.SetIndent(n)`) handles ALL indent decisions
- Continuation values are handled by the encoder automatically — it knows they're values, not keys
- No manual indent manipulation at all

Key insight: **yamlfmt delegates all indent logic to the go-yaml encoder.** It never touches whitespace directly.

### Both approaches share a common principle

**They use a STRUCTURAL understanding of the document (AST/Node tree) to determine indentation.** They do not try to infer structure from whitespace — they KNOW the structure from the parser, then emit correct whitespace from structure.

## Why Our Approach Is Different (And Harder)

Our tokenizer deliberately does NOT build a structural model. It classifies bytes into tokens. It doesn't know if `TokIndent + TokValue` is a continuation value or a nested structure. We chose this design to avoid the instability of AST serializers (goccy's `file.String()`).

The tradeoff: we get lossless tokenization and stable output, but we lose structural knowledge. And without structural knowledge, we can't correctly reindent.

## Options

### Option A: Add structural awareness to the depth computation

Enhance `computeDepths` to look at what follows each `TokIndent`:
- `TokIndent` → `TokKey` → `TokColon` = structural (renormalize independently)
- `TokIndent` → `TokDash` = structural (renormalize independently)
- `TokIndent` → `TokComment` = structural (renormalize independently)
- `TokIndent` → `TokValue` (no colon on this line) = continuation (shift by parent delta)
- `TokIndent` → `TokBlockScalar` = already handled as opaque
- `TokIndent` → `TokFlow` = structural (it's a value but self-contained)
- `TokIndent` → `TokAnchor`/`TokAlias`/`TokTag` = context-dependent

Implementation:
- In `applyIndent`, after computing the new indent for structural tokens, track the delta
- For continuation tokens (TokValue without colon), apply the SAME delta as the most recent structural ancestor instead of computing depth×width

Pros: Minimal change to architecture. Tokenizer stays unchanged.
Cons: "What follows the indent" heuristic may miss edge cases. A TokValue followed by content on the NEXT line could still be misidentified.

### Option B: Tokenizer emits different indent types

Add `TokContIndent` (continuation indent) vs `TokIndent` (structural indent). The tokenizer determines which it is by looking at context:
- After a `TokColon` with no value on the same line, the next indented line is continuation
- After a `TokDash` with no content, the next indented line is continuation

Implementation: tokenizer gains context awareness about what the PREVIOUS line was.

Pros: Clean separation at the token level. Depth computation doesn't need heuristics.
Cons: More complex tokenizer. Must get the context tracking right.

### Option C: Join continuation values into the value token (like prettier)

When the tokenizer sees `key:\n  value`, make the entire value (including the newline and indent) part of one `TokValue` token:
- `TokKey("key")` + `TokColon(":\n")` + `TokValue("  value")`

Or even simpler: if a line after a key-with-empty-value is more indented and has no colon, absorb it into the preceding value (same as we do for block scalars).

Pros: Continuation values become opaque (like block scalars). No reindent issues.
Cons: Requires the tokenizer to look at previous-line context. The colon token would need to include the newline. Significant tokenizer restructuring.

### Option D: Use yaml.v3's Node tree for STRUCTURE, our tokenizer for SERIALIZATION

Hybrid approach:
- Parse with `yaml.Unmarshal` into `yaml.Node` tree (we already do this for validation)
- Walk the Node tree to identify which source ranges are structural vs continuation
- Pass this structural map to `computeDepths`
- Tokenizer still handles lossless serialization

Pros: Correct by construction — yaml.v3 KNOWS what's structure vs content.
Cons: Couples formatting to yaml.v3's parse tree. Need to map Node positions back to token indices.

## Recommendation

**Option A** is the pragmatic choice for now. The rule is simple:

> If the tokens following a `TokIndent` do NOT contain a `TokColon` before the next `TokNewline`, this is a continuation line. Apply parent delta, not independent depth.

This handles:
- `key:\n  continuation value` — no colon after indent → continuation ✓
- `key:\n  nested: value` — HAS colon after indent → structural ✓
- `key:\n  - item` — dash is structural → ✓
- `key:\n  # comment` — comment is structural → ✓
- `key:\n  "quoted value"` — no colon → continuation ✓

It fails for:
- Nothing I can think of in real config files. A line without `:` or `-` after indent is always a value continuation or a comment.

## Detailed Implementation (Option A)

### Change 1: Classify indent tokens as structural or continuation

In `applyIndent`, before modifying any indent:

```go
func classifyIndent(tokens []Token, i int) bool {
    // Returns true if the indent at position i is structural (key, dash, comment)
    // Returns false if it's continuation (value without colon on this line)
    for j := i + 1; j < len(tokens); j++ {
        switch tokens[j].Kind {
        case TokKey:
            return true  // followed by key → structural
        case TokDash:
            return true  // followed by dash → structural
        case TokComment:
            return true  // followed by comment → structural
        case TokDocStart, TokDocEnd, TokDirective:
            return true  // document markers → structural
        case TokNewline:
            return false // reached EOL without key/dash → continuation
        case TokSpace, TokAnchor, TokTag, TokAlias:
            continue     // skip inline tokens, keep looking
        default:
            return false // value, flow, block scalar without preceding key → continuation
        }
    }
    return false // EOF → continuation
}
```

### Change 2: Handle continuation indent in applyIndent

For continuation lines, instead of `depth × targetWidth`, use `parentIndent + (originalIndent - parentOriginalIndent)`. This preserves the relative offset from the parent while shifting the absolute position.

Actually simpler: continuation lines get the SAME new indent as their parent structural indent PLUS their relative offset from the original parent indent.

```
parentOriginalIndent = 4  (the key's line had 4 spaces)
parentNewIndent = 2       (the key's line now has 2 spaces)
delta = 2 - 4 = -2

continuationOriginalIndent = 6  (this value line had 6 spaces)
continuationNewIndent = 6 + delta = 4  (shift by same delta)
```

### Change 3: Track "last structural indent" for delta reference

As we process tokens in `applyIndent`, maintain `lastStructuralDelta` — the delta applied to the most recent structural indent token. Continuation tokens use this same delta.

## Test Strategy

- The fuzz corpus entries that fail today are the regression tests
- All existing fixtures must still pass
- 45s fuzz on both targets with zero failures

## Files

- `pkg/formatter/yamlfmt/printer.go` — `applyIndent` function, add `classifyIndent` helper

## Risk

Medium. The classification heuristic ("does this line have a colon?") is simple and correct for all real config files. Edge cases only arise with pathological inputs (values that happen to contain colon-space, which would look structural). But yaml.v3 validation catches truly invalid structures before we reach the formatter.
