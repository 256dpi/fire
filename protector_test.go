package fire

import (
	"testing"

	"github.com/labstack/echo"
)

func TestProtector(t *testing.T) {
	p := DefaultProtector()
	p.AllowMethodOverriding = true
	p.XFrameOptions = "DENY"

	r := echo.New()

	p.Register(r)

	// TODO: Make tests actually test something.
}
