package validator

import (
	"bytes"
	"fmt"

	"gopkg.in/ini.v1"
)

// IniValidator validates INI files. When ForbidDuplicateKeys is true,
// duplicate keys within the same section are reported as errors.
type IniValidator struct {
	ForbidDuplicateKeys bool
}

var _ Validator = IniValidator{}

func (v IniValidator) ValidateSyntax(b []byte) (bool, error) {
	opts := ini.LoadOptions{}
	if v.ForbidDuplicateKeys {
		opts.AllowShadows = true
	}

	f, err := ini.LoadSources(opts, blankINIContinuations(b))
	if err != nil {
		return false, err
	}

	if v.ForbidDuplicateKeys {
		if err := checkINIDuplicateKeys(f); err != nil {
			return false, err
		}
	}

	return true, nil
}

// blankINIContinuations empties lines indented deeper than the key line
// above them. Python configparser (.flake8, .coveragerc, tox.ini) reads
// such a line as part of that key's value, not as a key of its own.
func blankINIContinuations(b []byte) []byte {
	lines := bytes.SplitAfter(b, []byte("\n"))
	keyIndent := -1
	for i, line := range lines {
		content := bytes.TrimLeft(line, " \t")
		indent := len(line) - len(content)
		text := bytes.TrimRight(content, "\r\n")
		switch {
		case keyIndent >= 0 && indent > keyIndent && len(text) > 0:
			lines[i] = content[len(text):]
		case len(text) == 0 || text[0] == '#' || text[0] == ';' || text[0] == '[':
			keyIndent = -1
		default:
			keyIndent = indent
		}
	}
	return bytes.Join(lines, nil)
}

func checkINIDuplicateKeys(f *ini.File) error {
	for _, section := range f.Sections() {
		for _, key := range section.Keys() {
			shadows := key.ValueWithShadows()
			if len(shadows) > 1 {
				name := section.Name()
				if name == "DEFAULT" {
					return fmt.Errorf("duplicate key %q", key.Name())
				}
				return fmt.Errorf("duplicate key %q in section [%s]", key.Name(), name)
			}
		}
	}
	return nil
}
