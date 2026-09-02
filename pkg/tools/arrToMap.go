package tools

// ArrToMap converts a string array
// to a map with keys from the array
// and empty struct values, optimizing string presence checks.
func ArrToMap(arg ...string) map[string]struct{} {
	m := make(map[string]struct{}, len(arg))
	for _, item := range arg {
		m[item] = struct{}{}
	}
	return m
}
