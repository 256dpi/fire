package fire

func stringInList(str string, list []string) bool {
	for _, item := range list {
		if item == str {
			return true
		}
	}

	return false
}
