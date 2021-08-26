package roast

import (
	"testing"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/jsonapi/v2"
	"github.com/stretchr/testify/assert"
)

func TestConvertModel(t *testing.T) {
	id := coal.New()

	res, err := ConvertModel(&fooModel{
		Base:   coal.B(id),
		String: "String",
		One:    id,
	})
	assert.NoError(t, err)
	assert.Equal(t, &jsonapi.Resource{
		Type: "foos",
		ID:   id.Hex(),
		Attributes: jsonapi.Map{
			"string": "String",
			"bool": false,
		},
		Relationships: map[string]*jsonapi.Document{
			"one": {
				Data: &jsonapi.HybridResource{
					One: &jsonapi.Resource{
						Type: "foos",
						ID:   id.Hex(),
					},
				},
			},
			"opt-one": {
				Data: nil,
			},
			"many": {
				Data: nil,
			},
		},
	}, res)

	res, err = ConvertModel(&fooModel{
		Base:   coal.B(id),
		String: "String",
		One:    id,
		Many:   []coal.ID{},
	})
	assert.NoError(t, err)
	assert.Equal(t, &jsonapi.Resource{
		Type: "foos",
		ID:   id.Hex(),
		Attributes: jsonapi.Map{
			"string": "String",
			"bool": false,
		},
		Relationships: map[string]*jsonapi.Document{
			"one": {
				Data: &jsonapi.HybridResource{
					One: &jsonapi.Resource{
						Type: "foos",
						ID:   id.Hex(),
					},
				},
			},
			"opt-one": {
				Data: nil,
			},
			"many": {
				Data: &jsonapi.HybridResource{
					Many: []*jsonapi.Resource{},
				},
			},
		},
	}, res)

	res, err = ConvertModel(&fooModel{
		Base:   coal.B(id),
		String: "String",
		One:    id,
		OptOne: &id,
		Many:   []coal.ID{id},
	})
	assert.NoError(t, err)
	assert.Equal(t, &jsonapi.Resource{
		Type: "foos",
		ID:   id.Hex(),
		Attributes: jsonapi.Map{
			"string": "String",
			"bool": false,
		},
		Relationships: map[string]*jsonapi.Document{
			"one": {
				Data: &jsonapi.HybridResource{
					One: &jsonapi.Resource{
						Type: "foos",
						ID:   id.Hex(),
					},
				},
			},
			"opt-one": {
				Data: &jsonapi.HybridResource{
					One: &jsonapi.Resource{
						Type: "foos",
						ID:   id.Hex(),
					},
				},
			},
			"many": {
				Data: &jsonapi.HybridResource{
					Many: []*jsonapi.Resource{
						{
							Type: "foos",
							ID:   id.Hex(),
						},
					},
				},
			},
		},
	}, res)
}

func TestAssignResource(t *testing.T) {
	id := coal.New()

	var foo fooModel
	err := AssignResource(&foo, &jsonapi.Resource{})
	assert.NoError(t, err)
	assert.Equal(t, fooModel{}, foo)

	foo = fooModel{}
	err = AssignResource(&foo, &jsonapi.Resource{
		Attributes: jsonapi.Map{
			"string": "String",
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, fooModel{
		String: "String",
	}, foo)

	foo = fooModel{}
	err = AssignResource(&foo, &jsonapi.Resource{
		Relationships: map[string]*jsonapi.Document{
			"one": {
				Data: &jsonapi.HybridResource{
					One: &jsonapi.Resource{
						ID: id.Hex(),
					},
				},
			},
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, fooModel{
		One: id,
	}, foo)

	foo = fooModel{}
	err = AssignResource(&foo, &jsonapi.Resource{
		Relationships: map[string]*jsonapi.Document{
			"opt-one": {
				Data: nil,
			},
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, fooModel{}, foo)

	foo = fooModel{}
	err = AssignResource(&foo, &jsonapi.Resource{
		Relationships: map[string]*jsonapi.Document{
			"opt-one": {
				Data: &jsonapi.HybridResource{
					One: &jsonapi.Resource{
						ID: id.Hex(),
					},
				},
			},
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, fooModel{
		OptOne: &id,
	}, foo)

	foo = fooModel{}
	err = AssignResource(&foo, &jsonapi.Resource{
		Relationships: map[string]*jsonapi.Document{
			"many": {
				Data: &jsonapi.HybridResource{
					Many: []*jsonapi.Resource{},
				},
			},
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, fooModel{
		Many: []coal.ID{},
	}, foo)

	foo = fooModel{}
	err = AssignResource(&foo, &jsonapi.Resource{
		Relationships: map[string]*jsonapi.Document{
			"many": {
				Data: &jsonapi.HybridResource{
					Many: []*jsonapi.Resource{
						{
							ID: id.Hex(),
						},
					},
				},
			},
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, fooModel{
		Many: []coal.ID{id},
	}, foo)
}
