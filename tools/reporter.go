package tools

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"
)

// DefaultReporter returns a reporter that writes to stderr.
func DefaultReporter() func(error) {
	return NewReporter(os.Stderr)
}

// NewReporter returns a very basic reporter that writes errors and stack
// traces to the specified writer.
func NewReporter(out io.Writer) func(error) {
	return func(err error) {
		fmt.Fprintf(out, "===> Begin Error: %s\n", err.Error())
		out.Write(debug.Stack())
		fmt.Fprintln(out, "<=== End Error")
	}
}
