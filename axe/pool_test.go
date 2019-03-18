package axe

import (
	"testing"

	"github.com/globalsign/mgo/bson"
	"github.com/stretchr/testify/assert"
)

type data struct {
	Foo string `bson:"foo"`
}

func TestPool(t *testing.T) {
	q := &Queue{
		Store: tester.Store,
	}

	done := make(chan struct{})

	p := NewPool()
	p.Add(&Task{
		Name:  "foo",
		Model: &data{},
		Queue: q,
		Handler: func(m Model) (bson.M, error) {
			close(done)
			return nil, nil
		},
		Workers:     2,
		MaxAttempts: 2,
	})
	p.Run()

	store := tester.Store.Copy()
	defer store.Close()

	_, err := Enqueue(store, "foo", &data{
		Foo: "bar",
	}, 0)
	assert.NoError(t, err)

	<-done

	p.Close()
}
