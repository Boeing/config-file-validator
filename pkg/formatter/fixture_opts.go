package formatter

import (
	"encoding/json"
	"os"
)

// LoadFixtureOptions reads a JSON sidecar file to override formatting options.
// The file maps Options field names to values. Zero-value fields in the JSON
// are left at their default from baseOpts.
//
// Example opts.json:
//
//	{"IndentStyle": 2, "IndentWidth": 4}
//
// IndentStyle values: 0=IndentDefault, 1=IndentSpaces, 2=IndentTabs
// LineEnding values:  0=LineEndingDefault, 1=LineEndingLF, 2=LineEndingCRLF
// TrailingCommas values: 0=TrailingCommasPreserve, 1=TrailingCommasAll,
// 2=TrailingCommasNone
//
// If the file does not exist, baseOpts is returned unchanged.
func LoadFixtureOptions(path string, baseOpts Options) Options {
	data, err := os.ReadFile(path)
	if err != nil {
		return baseOpts
	}
	// Unmarshal into the base options so only specified fields are overridden.
	_ = json.Unmarshal(data, &baseOpts)
	return baseOpts
}
