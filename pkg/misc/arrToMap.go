package misc

func ArrToMap(arr []string) map[string]struct{} {
	m := make(map[string]struct{}, 0)
	for _, item := range arr {
		m[item] = struct{}{}
	}
	return m
}
