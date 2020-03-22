package stick

// Unique will return a new list with unique strings.
func Unique(list []string) []string {
	// prepare map and list
	m := make(map[string]bool)
	l := make([]string, 0, len(list))

	// add items not present in map
	for _, id := range list {
		if _, ok := m[id]; !ok {
			m[id] = true
			l = append(l, id)
		}
	}

	return l
}

// Contains return whether the list contains the item.
func Contains(list []string, str string) bool {
	for _, item := range list {
		if item == str {
			return true
		}
	}

	return false
}

// Includes returns whether a list includes another list.
func Includes(all, subset []string) bool {
	for _, item := range subset {
		if !Contains(all, item) {
			return false
		}
	}

	return true
}

// Union will will merge two lists and remove duplicates.
func Union(listA, listB []string) []string {
	// prepare new list
	list := make([]string, 0, len(listA)+len(listB))
	list = append(list, listA...)
	list = append(list, listB...)

	// return unique list
	return Unique(list)
}

// Intersect will return the intersection of two lists.
func Intersect(listA, listB []string) []string {
	// prepare new list
	list := make([]string, 0, len(listA))

	// add items that are part of both lists
	for _, item := range listA {
		if Contains(listB, item) {
			list = append(list, item)
		}
	}

	return list
}
