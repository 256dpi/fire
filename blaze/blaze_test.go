package blaze

import (
	"testing"

	"github.com/256dpi/fire/coal"
)

func TestJobController(t *testing.T) {
	JobController(nil)
}

func TestC(t *testing.T) {
	C(coal.MustCreateStore("mongodb://0.0.0.0/test-fire-blaze").Copy())
}
