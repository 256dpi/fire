package stick

// Unique will return a new list with unique strings.
func Unique(list []string) []string {
	// prepare table and result
	table := make(map[string]bool)
	res := make([]string, 0, len(list))

	// add items not in table
	for _, item := range list {
		if _, ok := table[item]; !ok {
			table[item] = true
			res = append(res, item)
		}
	}

	return res
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

// Union will merge all list and remove duplicates.
func Union(lists ...[]string) []string {
	// sum length
	var sum int
	for _, l := range lists {
		sum += len(l)
	}

	// prepare table and result
	table := make(map[string]bool, sum)
	res := make([]string, 0, sum)

	// add items not present in table
	for _, list := range lists {
		for _, item := range list {
			if _, ok := table[item]; !ok {
				table[item] = true
				res = append(res, item)
			}
		}
	}

	return res
}

// Subtract will return a list with items that are only part of the first list.
func Subtract(listA, listB []string) []string {
	// prepare new list
	list := make([]string, 0, len(listA))

	// add items that are not in second list
	for _, item := range listA {
		if !Contains(listB, item) {
			list = append(list, item)
		}
	}

	return list
}

// Intersect will return a list with items that are not part of both lists.
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
