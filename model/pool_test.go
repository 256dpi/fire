package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewStore(t *testing.T) {
	p := NewStore("mongodb://localhost/fire")

	sess, db, err := p.Get()
	assert.NotNil(t, sess)
	assert.NotNil(t, db)
	assert.NoError(t, err)
}

func TestNewStoreError(t *testing.T) {
	p := NewStore("mongodb://localhost/fire?make=fail")

	sess, db, err := p.Get()
	assert.Nil(t, sess)
	assert.Nil(t, db)
	assert.Error(t, err)
}
