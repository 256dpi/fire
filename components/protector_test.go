package components

import (
	"testing"

	"github.com/labstack/echo"
)

// TODO: Make tests actually test something.

func TestProtector(t *testing.T) {
	p := DefaultProtector()
	p.AllowMethodOverriding = true
	p.XFrameOptions = "DENY"

	r := echo.New()

	p.Register(r)
	p.Inspect()
}
