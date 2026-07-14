# Plan: XML Format Options (Part of XML CST Formatter)

## Options to Add

### 1. `whitespace-sensitivity` (critical — affects formatting behavior)

Controls how the formatter treats whitespace in XML content:
- **`preserve`** (default): Only reformat indentation between pure-element children. Leave mixed-content elements and text-only elements untouched. This is what we're building.
- **`ignore`**: Treat all whitespace as insignificant. Reformat everything, even compact single-line XML like `<root><child/></root>` → pretty-printed. This is what HTML formatters do.

This option determines whether our printer INSERTS newlines (ignore mode) or only MODIFIES existing indent (preserve mode).

**Impact on implementation**: In `preserve` mode, if the input has no newlines between tags, we don't add them. In `ignore` mode, we insert newlines between sibling elements and before/after children.

### 2. `self-closing-space` (cosmetic)

Controls the space before `/>` in self-closing tags:
- `true` (default): `<br />`
- `false`: `<br/>`

**Impact**: Post-processing pass on TokSelfClose tokens — add or remove space before `/>`.

### 3. `preserve-xml-declaration` (structural)

Controls whether `<?xml ...?>` is preserved:
- `true` (default): Keep if present, don't add if absent
- `false`: Strip it

**Impact**: If false, remove TokXMLDecl tokens during printing.

## Where to Put Them

### In `pkg/configfile/configfile.go`:

Add XML-specific options struct:
```go
type XMLFormatOptions struct {
    WhitespaceSensitivity *string `toml:"whitespace-sensitivity"` // preserve/ignore
    SelfClosingSpace      *bool   `toml:"self-closing-space"`
}
```

Add to FormatConfig:
```go
type FormatConfig struct {
    FormatOptions
    // ... existing per-format overrides ...
    XMLOptions *XMLFormatOptions `toml:"xml-options"`
}
```

### In `pkg/formatter/formatter.go`:

Add XML-specific fields to Options:
```go
type Options struct {
    // ... existing fields ...
    
    // XML-specific
    XMLWhitespaceSensitivity XMLWhitespace // preserve/ignore
    XMLSelfClosingSpace      bool          // space before />
}

type XMLWhitespace int

const (
    XMLWhitespacePreserve XMLWhitespace = iota
    XMLWhitespaceIgnore
)
```

### In the XML formatter:

The `printFormatted` function checks these options:
```go
func printFormatted(tokens []Token, opts formatter.Options, src []byte) []byte {
    annotate(tokens, src)
    
    if opts.XMLWhitespaceSensitivity == formatter.XMLWhitespaceIgnore {
        tokens = insertFormattingWhitespace(tokens)
    }
    
    reindentTokens(tokens, indent, opts.IndentWidth)
    
    if opts.XMLSelfClosingSpace {
        ensureSelfClosingSpace(tokens)
    } else {
        removeSelfClosingSpace(tokens)
    }
    
    return serialize(tokens, opts)
}
```

### `insertFormattingWhitespace` (ignore mode only):

This is the function that takes compact XML and makes it pretty:
1. After TokOpenTag, if next non-whitespace token is TokOpenTag/TokSelfClose/TokCloseTag/TokComment → insert TokNewline + TokIndent
2. Before TokCloseTag, if previous non-whitespace token is TokCloseTag/TokSelfClose/TokOpenTag → insert TokNewline + TokIndent
3. Between sibling elements → insert TokNewline + TokIndent
4. Remove whitespace-only TokText between elements (old indent)
5. Keep TokText with non-whitespace content (actual text content)

This is equivalent to etree's "stripIndent + re-insert indent" but at token level.

## Config resolution flow

```
.cfv.toml:
  [format.xml]
  indent = 2
  whitespace-sensitivity = "ignore"
  self-closing-space = true

CLI: cfv format --indent=4 .

Resolution:
  1. defaults: whitespace-sensitivity=preserve, self-closing-space=true
  2. .cfv.toml [format.xml] overrides → whitespace-sensitivity=ignore
  3. CLI --indent=4 overrides → indent=4
  
  Final: indent=4, whitespace-sensitivity=ignore, self-closing-space=true
```

## Implementation Order

1. Add `XMLWhitespace` type and fields to `formatter.Options` (2 min)
2. Add `XMLFormatOptions` to configfile (5 min)
3. Wire config resolution in `cmd/cfv/cfv.go` (5 min)
4. In XML printer: implement `preserve` mode (what we're building now)
5. In XML printer: implement `ignore` mode (`insertFormattingWhitespace`)
6. In XML printer: implement `self-closing-space` post-pass
7. Tests for both modes

## Default Behavior

Default is `preserve` mode — we only reformat existing indentation, never insert newlines. This is SAFE — it never changes document semantics for ANY XML file.

`ignore` mode is opt-in for users who know their XML is data-only (Maven POM, .csproj, plist) and want full pretty-printing.

This means our existing fixtures need to work in PRESERVE mode. The `basic.input.xml` test (compact single-line) would stay compact in preserve mode. Users who want it pretty-printed set `whitespace-sensitivity = "ignore"`.

## Fixture Update

Current expected output assumes helium's "always pretty-print" behavior. With preserve mode as default, we need to:
- Update fixture expectations to match preserve behavior
- Add new `ignore` mode fixtures that test full pretty-printing

OR: make the default `ignore` to match current test expectations. This is what prettier-xml defaults to for most XML types. Only XHTML/SVG default to "preserve."

**Decision needed**: Should the default be `preserve` (safe) or `ignore` (what users expect from a formatter)?

**Recommendation**: Default to `ignore` for config files. Config XML (POM, web.xml, .csproj) is never whitespace-significant. Users formatting XHTML can set `preserve`. This matches what prettier does.
