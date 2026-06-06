package validator

import (
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

	f, err := ini.LoadSources(opts, b)
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
