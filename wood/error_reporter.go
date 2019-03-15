package wood

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"
)

// DefaultErrorReporter returns a reporter that writes to stderr.
func DefaultErrorReporter() func(error) {
	return NewErrorReporter(os.Stderr)
}

// NewErrorReporter returns a very basic reporter that writes errors and stack
// traces to the specified writer.
func NewErrorReporter(out io.Writer) func(error) {
	return func(err error) {
		_, _ = fmt.Fprintf(out, "===> Begin Error: %s\n", err.Error())
		_, _ = out.Write(debug.Stack())
		_, _ = fmt.Fprintln(out, "<=== End Error")
	}
}
