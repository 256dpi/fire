package torch

import (
	"fmt"

	"github.com/256dpi/fire/axe"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

// ProcessJob defines a job that processes a single operation.
type ProcessJob struct {
	axe.Base  `json:"-" axe:"torch/process"`
	Operation string  `json:"operation"`
	Model     coal.ID `json:"model"`
}

// NewProcessJob creates and returns a new ProcessJob with a label.
func NewProcessJob(operation string, model coal.ID) *ProcessJob {
	return &ProcessJob{
		Base:      axe.B(fmt.Sprintf("%s-%s", operation, model.Hex())),
		Operation: operation,
		Model:     model,
	}
}

// Validate implements the axe.Job interface.
func (j *ProcessJob) Validate() error {
	return stick.Validate(j, func(v *stick.Validator) {
		v.Value("Operation", false, stick.IsNotZero)
		v.Value("Model", false, stick.IsNotZero)
	})
}

// ScanJob defines a job that scans for due operations.
type ScanJob struct {
	axe.Base  `json:"-" axe:"torch/scan"`
	Operation string `json:"operation"`
}

// NewScanJob creates and returns a new ScanJob with a label.
func NewScanJob(operation string) *ScanJob {
	return &ScanJob{
		Base:      axe.B(operation),
		Operation: operation,
	}
}

// Validate implements the axe.Job interface.
func (j *ScanJob) Validate() error {
	return nil
}
