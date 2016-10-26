package fire

import (
	"net/http"
	"strconv"
)

// TODO: Add CORS?

func DefaultProtector() func(http.Handler) http.Handler {
	return NewProtector("4K")
}

func NewProtector(maxBody string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			r.Body = http.MaxBytesReader(w, r.Body, parseHumanSize(maxBody))
			next.ServeHTTP(w, r)
		})
	}
}

func parseHumanSize(str string) int64 {
	const msg = "size must be like 4K, 20M or 5G"

	if len(str) < 2 {
		panic(msg)
	}

	sym := string(str[len(str)-1])

	num, err := strconv.Atoi(str[:len(str)-1])
	if err != nil {
		panic(msg)
	}

	switch sym {
	case "K":
		return int64(num) * 1000
	case "M":
		return int64(num) * 1000 * 1000
	case "G":
		return int64(num) * 1000 * 1000 * 1000
	default:
		panic(msg)
	}
}
