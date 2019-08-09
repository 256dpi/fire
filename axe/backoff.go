package axe

import (
	"math"
	"time"
)

const maxInt64 = float64(math.MaxInt64 - 512)

func backoff(min, max time.Duration, factor float64, attempt int) time.Duration {
	// set default min
	if min <= 0 {
		min = 100 * time.Millisecond
	}

	// set default max
	if max <= 0 {
		max = 10 * time.Second
	}

	// check min and max
	if min >= max {
		return max
	}

	// set default factor
	if factor <= 0 {
		factor = 2
	}

	// calculate exponential
	exp := float64(min) * math.Pow(factor, float64(attempt))

	// ensure we wont overflow int64
	if exp > maxInt64 {
		return max
	}

	// get duration
	dur := time.Duration(exp)

	// keep within bounds
	if dur < min {
		return min
	} else if dur > max {
		return max
	}

	return dur
}
