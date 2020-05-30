package stick

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestF(t *testing.T) {
	err := F("foo %d", 42)

	str := err.Error()
	assert.Equal(t, `foo 42`, str)

	str = fmt.Sprintf("%s", err)
	assert.Equal(t, `foo 42`, str)

	str = fmt.Sprintf("%v", err)
	assert.Equal(t, `foo 42`, str)

	str = fmt.Sprintf("%+v", err)
	assert.Equal(t, []string{
		"foo 42",
		"github.com/256dpi/fire/stick.F",
		"  /Users/256dpi/Development/GitHub/256dpi/fire/stick/errors.go:LN",
		"github.com/256dpi/fire/stick.TestF",
		"  /Users/256dpi/Development/GitHub/256dpi/fire/stick/errors_test.go:LN",
		"testing.tRunner",
		"  /usr/local/Cellar/go/1.14.1/libexec/src/testing/testing.go:LN",
		"runtime.goexit",
		"  /usr/local/Cellar/go/1.14.1/libexec/src/runtime/asm_amd64.s:LN",
	}, splitTrace(str))
}

func TestWF(t *testing.T) {
	err := F("foo")
	err = WF(err, "bar %d", 42)

	str := err.Error()
	assert.Equal(t, `bar 42: foo`, str)

	str = fmt.Sprintf("%s", err)
	assert.Equal(t, `bar 42: foo`, str)

	str = fmt.Sprintf("%v", err)
	assert.Equal(t, `bar 42: foo`, str)

	str = fmt.Sprintf("%+v", err)
	assert.Equal(t, []string{
		"foo",
		"github.com/256dpi/fire/stick.F",
		"  /Users/256dpi/Development/GitHub/256dpi/fire/stick/errors.go:LN",
		"github.com/256dpi/fire/stick.TestWF",
		"  /Users/256dpi/Development/GitHub/256dpi/fire/stick/errors_test.go:LN",
		"testing.tRunner",
		"  /usr/local/Cellar/go/1.14.1/libexec/src/testing/testing.go:LN",
		"runtime.goexit",
		"  /usr/local/Cellar/go/1.14.1/libexec/src/runtime/asm_amd64.s:LN",
		"bar 42",
		"github.com/256dpi/fire/stick.WF",
		"  /Users/256dpi/Development/GitHub/256dpi/fire/stick/errors.go:LN",
		"github.com/256dpi/fire/stick.TestWF",
		"  /Users/256dpi/Development/GitHub/256dpi/fire/stick/errors_test.go:LN",
		"testing.tRunner",
		"  /usr/local/Cellar/go/1.14.1/libexec/src/testing/testing.go:LN",
		"runtime.goexit",
		"  /usr/local/Cellar/go/1.14.1/libexec/src/runtime/asm_amd64.s:LN",
	}, splitTrace(str))
}

func TestE(t *testing.T) {
	err := E("foo")
	assert.True(t, IsSafe(err))

	str := err.Error()
	assert.Equal(t, `foo`, str)

	str = fmt.Sprintf("%s", err)
	assert.Equal(t, `foo`, str)

	str = fmt.Sprintf("%v", err)
	assert.Equal(t, `foo`, str)

	str = fmt.Sprintf("%+v", err)
	assert.Equal(t, []string{
		"foo",
		"github.com/256dpi/fire/stick.F",
		"  /Users/256dpi/Development/GitHub/256dpi/fire/stick/errors.go:LN",
		"github.com/256dpi/fire/stick.E",
		"  /Users/256dpi/Development/GitHub/256dpi/fire/stick/errors.go:LN",
		"github.com/256dpi/fire/stick.TestE",
		"  /Users/256dpi/Development/GitHub/256dpi/fire/stick/errors_test.go:LN",
		"testing.tRunner",
		"  /usr/local/Cellar/go/1.14.1/libexec/src/testing/testing.go:LN",
		"runtime.goexit",
		"  /usr/local/Cellar/go/1.14.1/libexec/src/runtime/asm_amd64.s:LN",
	}, splitTrace(str))

	/* wrapped */

	err = WF(err, "bar")
	assert.True(t, IsSafe(err))

	str = err.Error()
	assert.Equal(t, `bar: foo`, str)

	str = fmt.Sprintf("%s", err)
	assert.Equal(t, `bar: foo`, str)

	str = fmt.Sprintf("%v", err)
	assert.Equal(t, `bar: foo`, str)

	str = fmt.Sprintf("%+v", err)
	assert.Equal(t, []string{
		"foo",
		"github.com/256dpi/fire/stick.F",
		"  /Users/256dpi/Development/GitHub/256dpi/fire/stick/errors.go:LN",
		"github.com/256dpi/fire/stick.E",
		"  /Users/256dpi/Development/GitHub/256dpi/fire/stick/errors.go:LN",
		"github.com/256dpi/fire/stick.TestE",
		"  /Users/256dpi/Development/GitHub/256dpi/fire/stick/errors_test.go:LN",
		"testing.tRunner",
		"  /usr/local/Cellar/go/1.14.1/libexec/src/testing/testing.go:LN",
		"runtime.goexit",
		"  /usr/local/Cellar/go/1.14.1/libexec/src/runtime/asm_amd64.s:LN",
		"bar",
		"github.com/256dpi/fire/stick.WF",
		"  /Users/256dpi/Development/GitHub/256dpi/fire/stick/errors.go:LN",
		"github.com/256dpi/fire/stick.TestE",
		"  /Users/256dpi/Development/GitHub/256dpi/fire/stick/errors_test.go:LN",
		"testing.tRunner",
		"  /usr/local/Cellar/go/1.14.1/libexec/src/testing/testing.go:LN",
		"runtime.goexit",
		"  /usr/local/Cellar/go/1.14.1/libexec/src/runtime/asm_amd64.s:LN",
	}, splitTrace(str))
}

func TestSafeError(t *testing.T) {
	err1 := F("foo")
	assert.False(t, IsSafe(err1))
	assert.Equal(t, "foo", err1.Error())
	assert.Nil(t, AsSafe(err1))

	err2 := Safe(err1)
	assert.True(t, IsSafe(err2))
	assert.Equal(t, "foo", err2.Error())
	assert.Equal(t, err2, AsSafe(err2))

	err3 := WF(err2, "bar")
	assert.True(t, IsSafe(err3))
	assert.Equal(t, "bar: foo", err3.Error())
	assert.Equal(t, err2, AsSafe(err3))
}

func splitTrace(str string) []string {
	str = strings.ReplaceAll(str, "\t", "  ")
	str = regexp.MustCompile(":\\d+").ReplaceAllString(str, ":LN")
	return strings.Split(str, "\n")
}
