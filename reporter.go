package fire

import (
	"fmt"
	"io"
	"os"
)

var _ ReporterComponent = (*Reporter)(nil)

// A Reporter can be used write errors occurring during request to a writer.
type Reporter struct {
	Writer io.Writer
}

// DefaultReporter creates and returns a reporter that writes errors to stderr.
func DefaultReporter() *Reporter {
	return NewReporter(os.Stderr)
}

// NewReporter create and returns a new reporter writing to the specified writer.
func NewReporter(writer io.Writer) *Reporter {
	return &Reporter{
		Writer: writer,
	}
}

// Describe implements the Component interface.
func (r *Reporter) Describe() ComponentInfo {
	return ComponentInfo{
		Name: "Reporter",
	}
}

// Report implements the ReporterComponent interface.
func (r *Reporter) Report(err error) error {
	fmt.Fprintf(r.Writer, "Error: %s", err)
	return nil
}
