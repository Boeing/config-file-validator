package reporter

import "slices"

func groupNodeFromSingle(groupReports map[string][]Report) *GroupNode {
	root := &GroupNode{}
	for _, group := range sortedKeys(groupReports) {
		root.Children = append(root.Children, &GroupNode{
			Key:     group,
			Reports: groupReports[group],
		})
	}
	return root
}

func groupNodeFromDouble(groupReports map[string]map[string][]Report) *GroupNode {
	root := &GroupNode{}
	for _, group := range sortedKeys(groupReports) {
		child := &GroupNode{Key: group}
		for _, groupTwo := range sortedKeys(groupReports[group]) {
			child.Children = append(child.Children, &GroupNode{
				Key:     groupTwo,
				Reports: groupReports[group][groupTwo],
			})
		}
		root.Children = append(root.Children, child)
	}
	return root
}

func groupNodeFromTriple(groupReports map[string]map[string]map[string][]Report) *GroupNode {
	root := &GroupNode{}
	for _, group := range sortedKeys(groupReports) {
		child := &GroupNode{Key: group}
		for _, groupTwo := range sortedKeys(groupReports[group]) {
			grandchild := &GroupNode{Key: groupTwo}
			for _, groupThree := range sortedKeys(groupReports[group][groupTwo]) {
				grandchild.Children = append(grandchild.Children, &GroupNode{
					Key:     groupThree,
					Reports: groupReports[group][groupTwo][groupThree],
				})
			}
			child.Children = append(child.Children, grandchild)
		}
		root.Children = append(root.Children, child)
	}
	return root
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
