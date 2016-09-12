package fire

import (
	"testing"

	"github.com/labstack/echo"
)

func TestProtector(t *testing.T) {
	p := NewProtector()
	r := echo.New()

	p.Register(r)

	// TODO: Make tests actually test something.
}
