package fire

import "testing"

func TestGroup(t *testing.T) {
	group := NewGroup()
	group.Add(&Controller{
		Model: &Post{},
	})
}
