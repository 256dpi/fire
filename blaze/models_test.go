package blaze

import (
	"encoding/json"
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

	err := link.Validate()
	assert.Equal(t, "invalid file", err.Error())

	link.File = coal.P(coal.ID{})

	err = link.Validate()
	assert.Equal(t, "invalid file", err.Error())

	link.File = coal.P(coal.New())

	err = link.Validate()
	assert.Error(t, err)
	assert.Equal(t, "type invalid", err.Error())

	link.FileType = "foo/bar"

	err = link.Validate()
	assert.Error(t, err)
	assert.Equal(t, "zero size", err.Error())

	err = link.Validate("bar/foo")
	assert.Error(t, err)
	assert.Equal(t, "type unallowed", err.Error())

	link.FileSize = 12

	err = link.Validate()
	assert.NoError(t, err)
}

func TestLinksUnmarshal(t *testing.T) {
	links := Links{
		{Ref: "1", Size: 1},
		{Ref: "2", Size: 2},
		{Ref: "3", Size: 3},
	}

	err := json.Unmarshal([]byte(`[
		{ "ref": "2" },
		{ "ref": "3" },
		{ "ref": "1" }
	]`), &links)
	assert.NoError(t, err)
	assert.Equal(t, Links{
		{Ref: "2", Size: 2},
		{Ref: "3", Size: 3},
		{Ref: "1", Size: 1},
	}, links)
}
