package jsonapi

import "testing"

func TestGroup(t *testing.T) {
	group := NewGroup("foo")
	group.Add(&Controller{
		Model: &Post{},
	})
}
