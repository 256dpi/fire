package roast

import (
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

var counter int64

// N will return a unique number.
func N() int64 {
	return atomic.AddInt64(&counter, 1)
}

// S will replace all # with a unique number and return the string.
func S(str string) string {
	// check string
	if !strings.ContainsRune(str, '#') {
		str += "#"
	}

	// replace
	str = strings.ReplaceAll(str, "#", strconv.FormatInt(N(), 10))

	return str
}

// T will return a timestamp for a time like "Jul 16 16:16:16".
func T(t string) time.Time {
	// parse time
	ts, err := time.Parse(time.Stamp, t)
	if err != nil {
		panic(err)
	}

	// add year
	ts = ts.AddDate(time.Now().Year(), 0, 0)

	return ts
}

// Now returns the time in UTC and second precision to ensure encoding/decoding
// stability.
func Now() time.Time {
	return time.Now().Truncate(time.Second).UTC()
}
