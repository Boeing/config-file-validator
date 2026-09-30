package fixer

import (
	"bytes"
	"encoding/json"
)

// schemaTypeMap parses a JSON Schema and returns a map of dotted paths to their expected types.
// Example: {"properties": {"port": {"type": "integer"}}} → {"port": "integer"}
// Nested: {"properties": {"db": {"type": "object", "properties": {"port": {"type": "integer"}}}}} → {"db.port": "integer"}
func schemaTypeMap(schema []byte) map[string]string {
	var s map[string]json.RawMessage
	if err := json.Unmarshal(schema, &s); err != nil {
		return nil
	}

	result := make(map[string]string)
	extractProperties(s, "", result)
	return result
}

// extractProperties recursively walks schema properties and builds the type map.
func extractProperties(schema map[string]json.RawMessage, prefix string, result map[string]string) {
	propsRaw, ok := schema["properties"]
	if !ok {
		return
	}

	var props map[string]json.RawMessage
	if err := json.Unmarshal(propsRaw, &props); err != nil {
		return
	}

	for key, valRaw := range props {
		var propSchema map[string]json.RawMessage
		if err := json.Unmarshal(valRaw, &propSchema); err != nil {
			continue
		}

		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		// Record the type for this path.
		if typeRaw, hasType := propSchema["type"]; hasType {
			var typeName string
			if err := json.Unmarshal(typeRaw, &typeName); err == nil {
				result[path] = typeName
			}
		}

		// Recurse into nested objects.
		if _, hasProps := propSchema["properties"]; hasProps {
			extractProperties(propSchema, path, result)
		}
	}
}

// valueLocation represents a scalar value found in JSON source with its path and byte position.
type valueLocation struct {
	path  string // dotted path (e.g., "db.port")
	raw   []byte // raw bytes in source (e.g., `"8080"`)
	start int    // byte offset of value start (inclusive)
	end   int    // byte offset of value end (exclusive)
}

// jsonValueLocations walks JSON source and returns locations of all scalar values
// within objects (not arrays). Each location includes the path, the raw value bytes,
// and byte offsets in the original source.
func jsonValueLocations(src []byte) []valueLocation {
	w := &jsonWalker{src: src}
	w.walk()
	return w.locations
}

// jsonWalker is a state-machine based JSON scanner that tracks paths and byte offsets.
type jsonWalker struct {
	src       []byte
	pos       int
	locations []valueLocation
}

func (w *jsonWalker) walk() {
	w.skipWhitespace()
	if w.pos < len(w.src) {
		w.walkValue("")
	}
}

func (w *jsonWalker) walkValue(path string) {
	w.skipWhitespace()
	if w.pos >= len(w.src) {
		return
	}

	switch w.src[w.pos] {
	case '{':
		w.walkObject(path)
	case '[':
		w.walkArray(path)
	case '"':
		start := w.pos
		w.skipString()
		end := w.pos
		if path != "" {
			w.locations = append(w.locations, valueLocation{
				path:  path,
				raw:   w.src[start:end],
				start: start,
				end:   end,
			})
		}
	default:
		// number, bool, null — record position
		start := w.pos
		w.skipScalar()
		end := w.pos
		if path != "" {
			w.locations = append(w.locations, valueLocation{
				path:  path,
				raw:   w.src[start:end],
				start: start,
				end:   end,
			})
		}
	}
}

func (w *jsonWalker) walkObject(path string) {
	w.pos++ // skip '{'
	w.skipWhitespace()

	for w.pos < len(w.src) && w.src[w.pos] != '}' {
		// Read key.
		w.skipWhitespace()
		if w.pos >= len(w.src) || w.src[w.pos] != '"' {
			return
		}
		key := w.readStringContent()

		// Skip colon.
		w.skipWhitespace()
		if w.pos < len(w.src) && w.src[w.pos] == ':' {
			w.pos++
		}

		// Build child path.
		childPath := key
		if path != "" {
			childPath = path + "." + key
		}

		// Walk value.
		w.walkValue(childPath)

		// Skip comma.
		w.skipWhitespace()
		if w.pos < len(w.src) && w.src[w.pos] == ',' {
			w.pos++
		}
	}

	if w.pos < len(w.src) {
		w.pos++ // skip '}'
	}
}

func (w *jsonWalker) walkArray(_ string) {
	w.pos++ // skip '['
	w.skipWhitespace()

	for w.pos < len(w.src) && w.src[w.pos] != ']' {
		// Arrays don't contribute to the dotted path for schema matching.
		w.walkValue("")
		w.skipWhitespace()
		if w.pos < len(w.src) && w.src[w.pos] == ',' {
			w.pos++
		}
		w.skipWhitespace()
	}

	if w.pos < len(w.src) {
		w.pos++ // skip ']'
	}
}

// readStringContent reads a JSON string and returns the unescaped key content.
// Advances pos past the closing quote.
func (w *jsonWalker) readStringContent() string {
	if w.pos >= len(w.src) || w.src[w.pos] != '"' {
		return ""
	}
	w.pos++ // skip opening "

	var buf bytes.Buffer
	for w.pos < len(w.src) {
		ch := w.src[w.pos]
		if ch == '\\' {
			w.pos++
			if w.pos < len(w.src) {
				esc := w.src[w.pos]
				switch esc {
				case 'n':
					buf.WriteByte('\n')
				case 't':
					buf.WriteByte('\t')
				case 'r':
					buf.WriteByte('\r')
				default:
					buf.WriteByte(esc)
				}
				w.pos++
			}
			continue
		}
		if ch == '"' {
			w.pos++ // skip closing "
			return buf.String()
		}
		buf.WriteByte(ch)
		w.pos++
	}
	return buf.String()
}

// skipString advances pos past a JSON string (including quotes).
func (w *jsonWalker) skipString() {
	w.pos++ // skip opening "
	for w.pos < len(w.src) {
		if w.src[w.pos] == '\\' {
			w.pos += 2
			continue
		}
		if w.src[w.pos] == '"' {
			w.pos++ // skip closing "
			return
		}
		w.pos++
	}
}

// skipScalar advances pos past a number, bool, or null value.
func (w *jsonWalker) skipScalar() {
	for w.pos < len(w.src) {
		ch := w.src[w.pos]
		if ch == ',' || ch == '}' || ch == ']' || isJSONWhitespace(ch) {
			return
		}
		w.pos++
	}
}

func (w *jsonWalker) skipWhitespace() {
	for w.pos < len(w.src) && isJSONWhitespace(w.src[w.pos]) {
		w.pos++
	}
}
