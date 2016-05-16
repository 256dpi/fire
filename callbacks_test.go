package fire

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestCombine(t *testing.T) {
	var counter int

	cb := func(ctx *Context)(error, error) {
		counter++
		return nil, nil
	}

	ccb := Combine(cb, cb, cb)

	err, sysErr := ccb(nil)
	assert.NoError(t, err)
	assert.NoError(t, sysErr)
	assert.Equal(t, 3, counter)
}

func TestDependentResourcesValidator(t *testing.T) {
	db := getDB()

	// create validator
	validator := DependentResourcesValidator(map[string]string{
		"comments": "post_id",
	})

	// create post
	post1 := saveModel(db, "posts", &Post{})

	// call validator
	err, sysErr := validator(&Context{
		ID: post1.getBase().ID,
		DB: db,
	})

	assert.NoError(t, err)
	assert.NoError(t, sysErr)

	// create comment
	saveModel(db, "comments", &Comment{
		PostID: post1.getBase().ID,
	})

	// call validator
	err, sysErr = validator(&Context{
		ID: post1.getBase().ID,
		DB: db,
	})

	assert.Error(t, err)
	assert.NoError(t, sysErr)
}
