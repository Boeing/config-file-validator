package formatter

// NormalizeLineEndings converts line endings in data to the requested style.
//
// LineEndingDefault and LineEndingLF are both treated as LF (no-ops if the
// input already uses LF). LineEndingCRLF replaces bare \n with \r\n,
// leaving existing \r\n sequences untouched.
func NormalizeLineEndings(data []byte, ending LineEnding) []byte {
	if ending != LineEndingCRLF {
		return data
	}
	// Replace bare \n with \r\n (skip already-CRLF sequences).
	result := make([]byte, 0, len(data)+len(data)/10)
	for i, b := range data {
		if b == '\n' && (i == 0 || data[i-1] != '\r') {
			result = append(result, '\r')
		}
		result = append(result, b)
	}
	return result
}
