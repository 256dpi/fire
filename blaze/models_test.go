package blaze

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

func TestAddFileIndexes(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		catalog := coal.NewCatalog()
		AddFileIndexes(catalog)
		assert.NoError(t, catalog.EnsureIndexes(tester.Store))
		assert.NoError(t, catalog.EnsureIndexes(tester.Store))
	})
}

func TestValidateType(t *testing.T) {
	err := ValidateType("")
	assert.Error(t, err)
	assert.Equal(t, "type invalid", err.Error())

	err = ValidateType("image/png")
	assert.NoError(t, err)

	err = ValidateType("image/png", "image/jpeg")
	assert.Error(t, err)
	assert.Equal(t, "type unallowed", err.Error())

	err = ValidateType("text/html; charset=utf-8")
	assert.Error(t, err)
	assert.Equal(t, "type ambiguous", err.Error())
}

func TestLinkValidate(t *testing.T) {
	link := &Link{}

	err := link.Validate("foo")
	assert.Equal(t, "foo invalid file", err.Error())

	link.File = coal.P(coal.ID{})

	err = link.Validate("foo")
	assert.Equal(t, "foo invalid file", err.Error())

	link.File = coal.P(coal.New())

	err = link.Validate("foo")
	assert.Error(t, err)
	assert.Equal(t, "foo type invalid", err.Error())

	link.FileType = "foo/bar"

	err = link.Validate("foo")
	assert.Error(t, err)
	assert.Equal(t, "foo zero size", err.Error())

	err = link.Validate("foo", "bar/foo")
	assert.Error(t, err)
	assert.Equal(t, "foo type unallowed", err.Error())

	link.FileSize = 12

	err = link.Validate("foo")
	assert.NoError(t, err)
}
