package fire

import "strconv"

// DataSize parses human readable data sizes (e.g. 4K, 20M or 5G) and returns
// the amount of bytes they represent.
func DataSize(str string) uint64 {
	const msg = "fire: data size must be like 4K, 20M or 5G"

	// check length
	if len(str) < 2 {
		panic(msg)
	}

	// get symbol
	sym := string(str[len(str)-1])

	// parse number
	num, err := strconv.ParseUint(str[:len(str)-1], 10, 64)
	if err != nil {
		panic(msg)
	}

	// calculate size
	switch sym {
	case "K":
		return num * 1000
	case "M":
		return num * 1000 * 1000
	case "G":
		return num * 1000 * 1000 * 1000
	}

	panic(msg)
}
