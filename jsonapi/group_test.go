package jsonapi

import (
	"testing"

	"github.com/gonfire/fire"
	"github.com/stretchr/testify/assert"
)

func TestGroup(t *testing.T) {
	group := NewGroup("foo")
	group.Add(&Controller{
		Model: &Post{},
	})

	assert.Equal(t, fire.ComponentInfo{
		Name: "JSON API Group",
		Settings: fire.Map{
			"Prefix":    "foo",
			"Resources": "posts",
		},
	}, group.Describe())
}
