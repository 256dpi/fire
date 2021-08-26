package roast

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
	"github.com/256dpi/jsonapi/v2"
	"github.com/stretchr/testify/assert"
)

type fooModel struct {
	coal.Base `json:"-" bson:",inline" coal:"foos"`
	Hello     string    `json:"hello"`
	One       coal.ID   `json:"-" coal:"one:foos"`
	OptOne    *coal.ID  `json:"-" coal:"opt-one:foos"`
	Many      []coal.ID `json:"-" coal:"many:foos"`

	stick.NoValidation `json:"-" bson:"-"`
}

func TestNow(t *testing.T) {
	now1 := Now()
	buf, err := json.Marshal(now1)
	assert.NoError(t, err)

	var now2 time.Time
	err = json.Unmarshal(buf, &now2)
	assert.NoError(t, err)

	assert.Equal(t, now1, now2)
}

func TestConvertModel(t *testing.T) {
	id := coal.New()

	res, err := ConvertModel(&fooModel{
		Base:  coal.B(id),
		Hello: "Hello",
		One:   id,
	})
	assert.NoError(t, err)
	assert.Equal(t, &jsonapi.Resource{
		Type: "foos",
		ID:   id.Hex(),
		Attributes: jsonapi.Map{
			"hello": "Hello",
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
		Hello:  "Hello",
		One:    id,
		OptOne: &id,
		Many:   []coal.ID{id},
	})
	assert.NoError(t, err)
	assert.Equal(t, &jsonapi.Resource{
		Type: "foos",
		ID:   id.Hex(),
		Attributes: jsonapi.Map{
			"hello": "Hello",
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
			"hello": "Hello",
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, fooModel{
		Hello: "Hello",
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
				Data: &jsonapi.HybridResource{},
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
