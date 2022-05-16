package fire

import (
	"testing"

	"github.com/256dpi/jsonapi/v2"
	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

type testModel struct {
	coal.Base `json:"-" bson:",inline" coal:"foos"`
	String    string    `json:"string"`
	Bool      bool      `json:"bool"`
	One       coal.ID   `json:"-" coal:"one:foos"`
	OptOne    *coal.ID  `json:"-" coal:"opt-one:foos"`
	Many      []coal.ID `json:"-" coal:"many:foos"`

	stick.NoValidation `json:"-" bson:"-"`
}

func TestConvertModel(t *testing.T) {
	id := coal.New()

	res, err := ConvertModel(&testModel{
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
			"bool":   false,
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

	res, err = ConvertModel(&testModel{
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
			"bool":   false,
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

	res, err = ConvertModel(&testModel{
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
			"bool":   false,
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

	var foo testModel
	err := AssignResource(&foo, &jsonapi.Resource{})
	assert.NoError(t, err)
	assert.Equal(t, testModel{}, foo)

	foo = testModel{}
	err = AssignResource(&foo, &jsonapi.Resource{
		Attributes: jsonapi.Map{
			"string": "String",
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, testModel{
		String: "String",
	}, foo)

	foo = testModel{}
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
	assert.Equal(t, testModel{
		One: id,
	}, foo)

	foo = testModel{}
	err = AssignResource(&foo, &jsonapi.Resource{
		Relationships: map[string]*jsonapi.Document{
			"opt-one": {
				Data: nil,
			},
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, testModel{}, foo)

	foo = testModel{}
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
	assert.Equal(t, testModel{
		OptOne: &id,
	}, foo)

	foo = testModel{}
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
	assert.Equal(t, testModel{
		Many: nil,
	}, foo)

	foo = testModel{}
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
	assert.Equal(t, testModel{
		Many: []coal.ID{id},
	}, foo)
}
