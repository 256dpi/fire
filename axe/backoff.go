package axe

import (
	"math"
	"time"
)

const maxInt64 = float64(math.MaxInt64 - 512)

// Backoff will calculate the exponential delay.
func Backoff(min, max time.Duration, factor float64, attempt int) time.Duration {
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

	// calculate delay
	delay := float64(min) * math.Pow(factor, float64(attempt))
	if delay > maxInt64 {
		delay = maxInt64
	}

	// get duration
	duration := time.Duration(delay)
	if duration < min {
		duration = min
	} else if duration > max {
		duration = max
	}

	return duration
}
