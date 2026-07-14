# Plan: XML CST Formatter (Custom Tokenizer + Structural Map)

## Architecture

Same pattern as YAML:
- **helium** → parse DOM, structural map (element boundaries, mixed content detection)
- **Custom tokenizer** → lossless byte-level token stream
- **Custom printer** → reindent structural elements, preserve mixed content verbatim

## Why This Solves the helium Bugs

1. **StripBlanks entity encoding bug** → gone. We never call StripBlanks. Our tokenizer preserves entities verbatim.
2. **Writer.Format mixed content bug** → gone. We never call Writer.Format. We identify mixed-content elements via DOM and leave their content untouched.
3. **No ErrSkipped for mixed content** → we handle it correctly by preserving it.

## Token Types

```
TokXMLDecl        // <?xml version="1.0" encoding="UTF-8"?>
TokDoctype        // <!DOCTYPE ...>
TokProcInst       // <?target instruction?>
TokComment        // <!-- comment -->
TokCDATA          // <![CDATA[...]]>
TokOpenTag        // <element attr="value">
TokCloseTag       // </element>
TokSelfClose      // <element attr="value"/>
TokText           // text content between tags
TokIndent         // leading whitespace at start of line (the thing we modify)
TokNewline        // \n or \r\n
```

Key design:
- **TokIndent** is separate from **TokText** — only TokIndent gets modified
- **TokOpenTag** includes the full tag with all attributes (opaque)
- **TokText** is any non-whitespace text content between tags
- Tags are opaque — we don't parse attributes, we just classify boundaries

## Structural Map from helium DOM

Walk the helium DOM. For each element, determine:
- **pure element content** → children are only elements (+ whitespace). These get reindented.
- **mixed content** → children include both elements AND non-whitespace text. Preserve verbatim.
- **text-only content** → leaf element with only text. Preserve verbatim.

Output: for each element, record its line number and whether its CHILDREN should be reindented.

## How etree Does It (Reference)

etree's `indent()` function:
1. Strips all whitespace-only CharData (existing indent)
2. For each non-CharData child, inserts `\n` + spaces before it
3. CharData (text) nodes are NOT considered for indent insertion — they stay inline
4. Recursion: child elements get `depth+1`

Key insight: **etree skips indentation when CharData (non-whitespace) is present among element children.** That's mixed content detection — implicit.

Our tokenizer equivalent: if a TokText token exists between TokOpenTag and TokCloseTag at the same level as child elements → mixed content → don't reindent that scope.

## Tokenizer Design

The XML tokenizer is simpler than YAML because:
- No indent-is-structure ambiguity (indent is purely cosmetic in XML)
- Tags explicitly delimit structure (`<` and `>`)
- No continuation values or block scalars
- Comments and CDATA have explicit delimiters

The tokenizer just needs to:
1. At line start: consume whitespace as TokIndent
2. Detect `<` → classify as open tag, close tag, self-close, comment, CDATA, PI, doctype, or XML decl
3. Detect text content between tags

### Hard parts:
- **CDATA boundary** — `<![CDATA[...]]>` can contain anything including `<` and `>`
- **Comments** — `<!-- ... -->` can span multiple lines
- **Processing instructions** — `<?...?>` 
- **Attribute values with `>`** — `<tag attr="a>b">` must not split at the `>`

## Printer Design

```go
func printFormatted(tokens []Token, opts Options, src []byte) []byte {
    annotate(tokens, src)     // set Structural + Depth from helium DOM
    reindentTokens(tokens, width)
    return serialize(tokens)
}
```

`annotate` walks the helium DOM:
- For each element at depth D, mark TokIndent before its open/close tags as structural, depth=D
- For elements whose parent is mixed-content, mark all children's indents as NON-structural

`reindentTokens`:
- Structural TokIndent: `Raw = depth × width` spaces
- Non-structural TokIndent: shift by parent delta (same as YAML continuations)

**SortKeys** is not applicable to XML (no concept of key ordering in elements).

## Comparison with prettier

prettier formats XML via `@prettier/plugin-xml`. It uses a similar approach:
- Parse with `@xml-tools/parser` (CST-based)
- Reconstruct output from the CST with indent control
- Mixed content: preserves inline text, only indents element-only children

Our output should match prettier's XML formatting for standard config files (POM, web.xml, plist, Spring).

## Tasks

- [ ] 1. Implement XML tokenizer (`pkg/formatter/xmlfmt/tokenizer.go`)
  - Token types: XMLDecl, Doctype, ProcInst, Comment, CDATA, OpenTag, CloseTag, SelfClose, Text, Indent, Newline
  - Handle: CDATA boundaries, multi-line comments, attributes with >, self-closing tags
  - Every byte in exactly one token (losslessness invariant)
  - Fuzz tokenizer alone: no panics, losslessness holds

- [ ] 2. Implement structural annotation (`pkg/formatter/xmlfmt/printer.go`)
  - Parse with helium to get DOM
  - Walk DOM, identify mixed-content elements
  - Map element positions back to token indices (by line number)
  - Set Structural=true on indent tokens for pure-element-content parents
  - Set Structural=false (preserve) for mixed-content children

- [ ] 3. Implement printer (`pkg/formatter/xmlfmt/printer.go`)
  - Reindent structural indents: depth × width
  - Non-structural: shift by parent delta
  - Serialize: concatenate, strip trailing whitespace, final newline
  - No SortKeys for XML

- [ ] 4. Replace current xml.go Format function
  - Remove helium Writer usage
  - Remove ErrSkipped for mixed content (we handle it now)
  - Keep helium for DOM parse (structural map)
  - Keep helium for XSD validation (separate concern)

- [ ] 5. Tests
  - All existing fixtures pass (or improve)
  - New fixtures: mixed content, CDATA, comments, processing instructions, self-closing
  - Test against etree's Indent output for correctness reference
  - Fuzz: 45s minimum, zero failures

- [ ] 6. Pipeline verification + delete the known-failing fuzz corpus entry

## Dependencies

- **helium** — keep for DOM parsing and XSD validation (already in go.mod)
- **etree** — use as reference implementation for testing (add to go.mod or test-only)
- No new runtime dependencies for the formatter itself

## Effort Estimate

Based on YAML taking ~3 hours for tokenizer + printer + fuzz hardening:
- XML tokenizer is simpler (explicit delimiters, no indent-is-structure)
- No SortKeys complexity
- Mixed content is the main challenge, solved by the structural map

Estimate: **1-2 hours**

## Risk

Low. XML has explicit structure (tags). No ambiguity about what's content vs what's structure. The mixed-content detection via helium DOM is straightforward. And we've proven the pattern works with YAML.
