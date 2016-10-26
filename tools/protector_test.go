package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseHumanSize(t *testing.T) {
	assert.Equal(t, int64(50*1000), parseHumanSize("50K"))
	assert.Equal(t, int64(5*1000*1000), parseHumanSize("5M"))
	assert.Equal(t, int64(100*1000*1000*1000), parseHumanSize("100G"))

	for _, str := range []string{"", "1", "K", "10", "KM"} {
		assert.Panics(t, func() {
			parseHumanSize(str)
		})
	}
}

//import (
//	"testing"
//
//	"github.com/labstack/echo"
//)
//
//// TODO: Make tests actually test something.
//
//func TestProtector(t *testing.T) {
//	p := DefaultProtector()
//	p.AllowMethodOverriding = true
//
//	r := echo.New()
//
//	p.Register(r)
//	p.Describe()
//}
