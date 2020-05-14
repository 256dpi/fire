package coal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

func TestR(t *testing.T) {
	id := New()
	ref := R(&postModel{Base: B(id)})
	assert.Equal(t, Ref{
		Coll: "posts",
		ID:   id,
	}, ref)
}

func TestAnyRef(t *testing.T) {
	assert.Equal(t, bson.M{
		"$gte": Ref{Coll: "posts", ID: ID{}},
		"$lte": Ref{Coll: "posts", ID: maxID},
	}, AnyRef(&postModel{}))
}

func TestReferences(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		post := &postModel{Base: B()}
		note1 := &noteModel{Base: B()}
		note2 := &noteModel{Base: B()}

		poly1 := tester.Insert(&polyModel{
			Ref1: R(post),
		}).(*polyModel)
		poly2 := tester.Insert(&polyModel{
			Ref1: R(note1),
		}).(*polyModel)
		poly3 := tester.Insert(&polyModel{
			Ref1: R(note2),
		}).(*polyModel)

		var res []*polyModel
		err := tester.Store.M(&polyModel{}).FindAll(nil, &res, bson.M{
			"Ref1": R(post),
		}, nil, 0, 0, false, Unsafe)
		assert.NoError(t, err)
		assert.Equal(t, []*polyModel{
			poly1,
		}, res)

		res = nil
		err = tester.Store.M(&polyModel{}).FindAll(nil, &res, bson.M{
			"Ref1": R(note1),
		}, nil, 0, 0, false, Unsafe)
		assert.NoError(t, err)
		assert.Equal(t, []*polyModel{
			poly2,
		}, res)

		res = nil
		err = tester.Store.M(&polyModel{}).FindAll(nil, &res, bson.M{
			"Ref1": AnyRef(&noteModel{}),
		}, nil, 0, 0, false, Unsafe)
		assert.NoError(t, err)
		assert.Equal(t, []*polyModel{
			poly2,
			poly3,
		}, res)
	})
}
