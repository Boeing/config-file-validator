package tools

// IsGlobPattern reports whether s contains glob metacharacters.
func IsGlobPattern(s string) bool {
	for _, c := range s {
		if c == '*' || c == '?' || c == '[' {
			return true
		}
	}
	return false
}
