package fire

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClonePool(t *testing.T) {
	p := NewClonePool("mongodb://localhost/fire")

	sess, db, err := p.Get()
	assert.NotNil(t, sess)
	assert.NotNil(t, db)
	assert.NoError(t, err)
}

func TestClonePoolError(t *testing.T) {
	p := NewClonePool("mongodb://localhost/fire?make=fail")

	sess, db, err := p.Get()
	assert.Nil(t, sess)
	assert.Nil(t, db)
	assert.Error(t, err)
}
