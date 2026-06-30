package formatter

import "bytes"

// IsFormatted reports whether src is already in canonical form.
// This is a convenience function: it runs Format and compares the result
// byte-for-byte with the input.
func IsFormatted(f Formatter, src []byte, opts Options) (bool, error) {
	formatted, err := f.Format(src, opts)
	if err != nil {
		return false, err
	}
	return bytes.Equal(src, formatted), nil
}
