package blaze

import (
	"encoding/json"
	"testing"

	"github.com/256dpi/fire/stick"
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

	err := link.Validate(true)
	assert.Equal(t, "File: zero; FileName: zero; FileSize: too small; FileType: type invalid", err.Error())

	link.File = coal.New()
	link.FileName = "foo"
	link.FileType = "foo/bar"
	link.FileSize = 12

	err = link.Validate(true, "bar/foo")
	assert.Error(t, err)
	assert.Equal(t, "FileType: type unallowed", err.Error())

	err = link.Validate(true, "foo/bar")
	assert.NoError(t, err)
}

func TestIsValidLink(t *testing.T) {
	var model testModel
	err := stick.Validate(&model, func(v *stick.Validator) {
		v.Value("RequiredFile", false, IsValidLink(true))
	})
	assert.Equal(t, "RequiredFile.File: zero; RequiredFile.FileName: zero; RequiredFile.FileSize: too small; RequiredFile.FileType: type invalid", err.Error())

	model.RequiredFile = Link{
		File:     coal.New(),
		FileName: "foo",
		FileType: "foo/bar",
		FileSize: 12,
	}
	err = stick.Validate(&model, func(v *stick.Validator) {
		v.Value("RequiredFile", false, IsValidLink(true, "bar/foo"))
	})
	assert.Equal(t, "RequiredFile.FileType: type unallowed", err.Error())

	err = stick.Validate(&model, func(v *stick.Validator) {
		v.Value("RequiredFile", false, IsValidLink(true, "foo/bar"))
	})
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

func TestLinksValidate(t *testing.T) {
	links := Links{}
	err := links.Validate(true)
	assert.NoError(t, err)

	links = Links{{}}
	err = links.Validate(true)
	assert.Error(t, err)

	links = Links{
		{
			Ref:      "1",
			File:     coal.New(),
			FileType: "foo/bar",
			FileSize: 12,
		},
		{
			Ref:      "1",
			File:     coal.New(),
			FileType: "foo/bar",
			FileSize: 12,
		},
	}
	err = links.Validate(false)
	assert.Error(t, err)
	assert.Equal(t, "ambiguous reference", err.Error())

	links = Links{
		{
			Ref:      "1",
			File:     coal.New(),
			FileType: "foo/bar",
			FileSize: 12,
		},
		{
			Ref:      "2",
			File:     coal.New(),
			FileType: "foo/bar",
			FileSize: 12,
		},
	}
	err = links.Validate(true)
	assert.Error(t, err)
	assert.Equal(t, "FileName: zero", err.Error())

	links = Links{
		{
			Ref:      "1",
			File:     coal.New(),
			FileName: "foo",
			FileType: "foo/bar",
			FileSize: 12,
		},
		{
			Ref:      "2",
			File:     coal.New(),
			FileName: "bar",
			FileType: "foo/bar",
			FileSize: 12,
		},
	}
	err = links.Validate(true, "bar/foo")
	assert.Error(t, err)
	assert.Equal(t, "FileType: type unallowed", err.Error())

	err = links.Validate(true, "foo/bar")
	assert.NoError(t, err)
}

func TestIsLinksValid(t *testing.T) {
	var model testModel
	err := stick.Validate(&model, func(v *stick.Validator) {
		v.Value("MultipleFiles", false, IsValidLinks(true))
	})
	assert.NoError(t, err)

	model.MultipleFiles = Links{{}}
	err = stick.Validate(&model, func(v *stick.Validator) {
		v.Value("MultipleFiles", false, IsValidLinks(true))
	})
	assert.Error(t, err)

	model.MultipleFiles = Links{
		{
			Ref:      "1",
			File:     coal.New(),
			FileType: "foo/bar",
			FileSize: 12,
		},
		{
			Ref:      "1",
			File:     coal.New(),
			FileType: "foo/bar",
			FileSize: 12,
		},
	}
	err = stick.Validate(&model, func(v *stick.Validator) {
		v.Value("MultipleFiles", false, IsValidLinks(false))
	})
	assert.Error(t, err)
	assert.Equal(t, "MultipleFiles: ambiguous reference", err.Error())

	model.MultipleFiles = Links{
		{
			Ref:      "1",
			File:     coal.New(),
			FileType: "foo/bar",
			FileSize: 12,
		},
		{
			Ref:      "2",
			File:     coal.New(),
			FileType: "foo/bar",
			FileSize: 12,
		},
	}
	err = stick.Validate(&model, func(v *stick.Validator) {
		v.Value("MultipleFiles", false, IsValidLinks(true))
	})
	assert.Error(t, err)
	assert.Equal(t, "MultipleFiles.FileName: zero", err.Error())

	model.MultipleFiles = Links{
		{
			Ref:      "1",
			File:     coal.New(),
			FileName: "foo",
			FileType: "foo/bar",
			FileSize: 12,
		},
		{
			Ref:      "2",
			File:     coal.New(),
			FileName: "bar",
			FileType: "foo/bar",
			FileSize: 12,
		},
	}
	err = stick.Validate(&model, func(v *stick.Validator) {
		v.Value("MultipleFiles", false, IsValidLinks(true, "bar/foo"))
	})
	assert.Error(t, err)
	assert.Equal(t, "MultipleFiles.FileType: type unallowed", err.Error())

	err = stick.Validate(&model, func(v *stick.Validator) {
		v.Value("MultipleFiles", false, IsValidLinks(true, "foo/bar"))
	})
	assert.NoError(t, err)
}
