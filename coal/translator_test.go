package coal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

func TestTranslatorDocument(t *testing.T) {
	pt := Translate(&postModel{})

	// TODO: Nested doc equality?

	// unknown
	doc, err := pt.Document(bson.M{
		"foo": "bar",
	})
	assert.Error(t, err)
	assert.Nil(t, doc)
	assert.Equal(t, `unknown field "foo"`, err.Error())

	// known
	doc, err = pt.Document(bson.M{
		"title": "Hello World!",
	})
	assert.NoError(t, err)
	assert.Equal(t, bson.D{
		{Key: "title", Value: "Hello World!"},
	}, doc)

	// resolved
	doc, err = pt.Document(bson.M{
		"Title": "Hello World!",
	})
	assert.NoError(t, err)
	assert.Equal(t, bson.D{
		{Key: "title", Value: "Hello World!"},
	}, doc)

	// mixed
	doc, err = pt.Document(bson.M{
		"Title":     "Hello World!",
		"text_body": "This is awesome.",
	})
	assert.NoError(t, err)
	assert.Equal(t, bson.D{
		{Key: "title", Value: "Hello World!"},
		{Key: "text_body", Value: "This is awesome."},
	}, doc)

	// virtual
	doc, err = pt.Document(bson.M{
		"Title":    "Hello World!",
		"Comments": bson.A{},
	})
	assert.Error(t, err)
	assert.Nil(t, doc)
	assert.Equal(t, `virtual field "Comments"`, err.Error())

	// id
	id := New()
	doc, err = pt.Document(bson.M{
		"_id": id,
	})
	assert.NoError(t, err)
	assert.Equal(t, bson.D{
		{Key: "_id", Value: id},
	}, doc)

	// complex query
	doc, err = pt.Document(bson.M{
		"$or": bson.A{
			bson.M{
				"Title": "Hello World!",
			},
			bson.M{
				"Published": bson.M{
					"$eq": true,
				},
			},
		},
		"text_body": "This is awesome.",
	})
	assert.NoError(t, err)
	assert.Equal(t, bson.D{
		{Key: "$or", Value: bson.A{
			bson.D{
				{Key: "title", Value: "Hello World!"},
			},
			bson.D{
				{Key: "published", Value: bson.D{
					{Key: "$eq", Value: true},
				}},
			},
		}},
		{Key: "text_body", Value: "This is awesome."},
	}, doc)

	// complex update
	doc, err = pt.Document(bson.M{
		"$set": bson.M{
			"Title": "Hello World!",
		},
		"$setOnInsert": bson.M{
			"Published": false,
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "title", Value: "Hello World!"},
		}},
		{Key: "$setOnInsert", Value: bson.D{
			{Key: "published", Value: false},
		}},
	}, doc)

	// unsafe operator
	doc, err = pt.Document(bson.M{
		"$rename": bson.M{
			"Title": "Foo",
		},
	})
	assert.Error(t, err)
	assert.Nil(t, doc)
	assert.Equal(t, `unsafe operator "$rename"`, err.Error())

	// unsupported type
	doc, err = pt.Document(bson.M{
		"Title": []byte("foo"),
	})
	assert.Error(t, err)
	assert.Nil(t, doc)
	assert.Equal(t, `unsupported type []uint8`, err.Error())
}

func TestTranslatorSort(t *testing.T) {
	pt := Translate(&postModel{})

	// unknown
	doc, err := pt.Sort([]string{"foo", "-bar"})
	assert.Error(t, err)
	assert.Nil(t, doc)
	assert.Equal(t, `unknown field "foo"`, err.Error())

	// known
	doc, err = pt.Sort([]string{"title", "-text_body"})
	assert.NoError(t, err)
	assert.Equal(t, bson.D{
		{Key: "title", Value: int32(1)},
		{Key: "text_body", Value: int32(-1)},
	}, doc)

	// resolved
	doc, err = pt.Sort([]string{"Title", "-TextBody"})
	assert.NoError(t, err)
	assert.Equal(t, bson.D{
		{Key: "title", Value: int32(1)},
		{Key: "text_body", Value: int32(-1)},
	}, doc)

	// mixed
	doc, err = pt.Sort([]string{"Title", "-text_body"})
	assert.NoError(t, err)
	assert.Equal(t, bson.D{
		{Key: "title", Value: int32(1)},
		{Key: "text_body", Value: int32(-1)},
	}, doc)

	// virtual
	doc, err = pt.Sort([]string{"Title", "Comments"})
	assert.Error(t, err)
	assert.Nil(t, doc)
	assert.Equal(t, `virtual field "Comments"`, err.Error())

	// id
	doc, err = pt.Sort([]string{"_id"})
	assert.NoError(t, err)
	assert.Equal(t, bson.D{
		{Key: "_id", Value: int32(1)},
	}, doc)
}
