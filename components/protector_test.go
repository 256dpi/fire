package components

import (
	"testing"

	"github.com/labstack/echo"
)

// TODO: Make tests actually test something.

func TestProtector(t *testing.T) {
	p := DefaultProtector()
	p.AllowMethodOverriding = true

	r := echo.New()

	p.Register(r)
	p.Inspect()
}
