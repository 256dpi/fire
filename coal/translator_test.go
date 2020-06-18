package coal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestTranslatorDocument(t *testing.T) {
	trans := NewTranslator(&postModel{})

	// nil
	doc, err := trans.Document(nil)
	assert.NoError(t, err)
	assert.Equal(t, bson.D{}, doc)

	// unknown
	doc, err = trans.Document(bson.M{
		"foo": "bar",
	})
	assert.Error(t, err)
	assert.Nil(t, doc)
	assert.Equal(t, `unknown field "foo"`, err.Error())

	// known
	doc, err = trans.Document(bson.M{
		"title": "Hello World!",
	})
	assert.NoError(t, err)
	assert.Equal(t, bson.D{
		{Key: "title", Value: "Hello World!"},
	}, doc)

	// resolved
	doc, err = trans.Document(bson.M{
		"Title": "Hello World!",
	})
	assert.NoError(t, err)
	assert.Equal(t, bson.D{
		{Key: "title", Value: "Hello World!"},
	}, doc)

	// mixed
	doc, err = trans.Document(bson.M{
		"Title":     "Hello World!",
		"text_body": "This is awesome.",
	})
	assert.NoError(t, err)
	assert.Equal(t, bson.D{
		{Key: "title", Value: "Hello World!"},
		{Key: "text_body", Value: "This is awesome."},
	}, doc)

	// virtual
	doc, err = trans.Document(bson.M{
		"Title":    "Hello World!",
		"Comments": bson.A{},
	})
	assert.Error(t, err)
	assert.Nil(t, doc)
	assert.Equal(t, `virtual field "Comments"`, err.Error())

	// id
	id := New()
	doc, err = trans.Document(bson.M{
		"_id": id,
	})
	assert.NoError(t, err)
	assert.Equal(t, bson.D{
		{Key: "_id", Value: id},
	}, doc)

	// complex query
	doc, err = trans.Document(bson.M{
		"$or": bson.A{
			bson.M{
				// simple quality
				"Title": "Hello World!",

				// document equality
				"TextBody": bson.M{
					"foo": "bar",
				},
			},
			bson.M{
				// complex equality
				"Published": bson.M{
					"$eq": true,
				},

				// array equality
				"TextBody": bson.A{
					bson.M{
						"foo": "bar",
					},
				},
			},
		},
		"text_body": "This is awesome.",
	})
	assert.NoError(t, err)
	assert.Equal(t, bson.D{
		{Key: "$or", Value: bson.A{
			bson.D{
				{Key: "text_body", Value: bson.D{
					{Key: "foo", Value: "bar"},
				}},
				{Key: "title", Value: "Hello World!"},
			},
			bson.D{
				{Key: "published", Value: bson.D{
					{Key: "$eq", Value: true},
				}},
				{Key: "text_body", Value: bson.A{
					bson.D{
						{Key: "foo", Value: "bar"},
					},
				}},
			},
		}},
		{Key: "text_body", Value: "This is awesome."},
	}, doc)

	// complex update
	doc, err = trans.Document(bson.M{
		"$set": bson.M{
			// value
			"Title": "Hello World!",

			// document
			"TextBody": bson.M{
				"foo": "bar",
			},
		},
		"$setOnInsert": bson.M{
			// value
			"Published": false,

			// array
			"TextBody": bson.A{
				bson.M{
					"foo": "bar",
				},
			},
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "text_body", Value: bson.D{
				{Key: "foo", Value: "bar"},
			}},
			{Key: "title", Value: "Hello World!"},
		}},
		{Key: "$setOnInsert", Value: bson.D{
			{Key: "published", Value: false},
			{Key: "text_body", Value: bson.A{
				bson.D{
					{Key: "foo", Value: "bar"},
				},
			}},
		}},
	}, doc)

	// unsafe operator
	doc, err = trans.Document(bson.M{
		"$rename": bson.M{
			"Title": "Foo",
		},
	})
	assert.Error(t, err)
	assert.Nil(t, doc)
	assert.Equal(t, `unsafe operator "$rename"`, err.Error())

	// unsafe nested operator
	doc, err = trans.Document(bson.M{
		"Title": bson.M{
			"$where": "...",
		},
	})
	assert.Error(t, err)
	assert.Nil(t, doc)
	assert.Equal(t, `unsafe operator "$where"`, err.Error())

	// special type
	doc, err = trans.Document(bson.M{
		"Title": []byte("foo"),
	})
	assert.NoError(t, err)
	assert.Equal(t, bson.D{
		{Key: "title", Value: primitive.Binary{Data: []byte("foo")}},
	}, doc)

	// dotted fields
	doc, err = trans.Document(bson.M{
		"Title.foo": "bar",
	})
	assert.Error(t, err)
	assert.Nil(t, doc)
	assert.Equal(t, `unknown field "Title.foo"`, err.Error())
}

func TestTranslatorSort(t *testing.T) {
	trans := NewTranslator(&postModel{})

	// nil
	doc, err := trans.Sort(nil)
	assert.NoError(t, err)
	assert.Nil(t, doc)

	// unknown
	doc, err = trans.Sort([]string{"foo", "-bar"})
	assert.Error(t, err)
	assert.Nil(t, doc)
	assert.Equal(t, `unknown field "foo"`, err.Error())

	// known
	doc, err = trans.Sort([]string{"title", "-text_body"})
	assert.NoError(t, err)
	assert.Equal(t, bson.D{
		{Key: "title", Value: int32(1)},
		{Key: "text_body", Value: int32(-1)},
	}, doc)

	// resolved
	doc, err = trans.Sort([]string{"Title", "-TextBody"})
	assert.NoError(t, err)
	assert.Equal(t, bson.D{
		{Key: "title", Value: int32(1)},
		{Key: "text_body", Value: int32(-1)},
	}, doc)

	// mixed
	doc, err = trans.Sort([]string{"Title", "-text_body"})
	assert.NoError(t, err)
	assert.Equal(t, bson.D{
		{Key: "title", Value: int32(1)},
		{Key: "text_body", Value: int32(-1)},
	}, doc)

	// virtual
	doc, err = trans.Sort([]string{"Title", "Comments"})
	assert.Error(t, err)
	assert.Nil(t, doc)
	assert.Equal(t, `virtual field "Comments"`, err.Error())

	// id
	doc, err = trans.Sort([]string{"_id"})
	assert.NoError(t, err)
	assert.Equal(t, bson.D{
		{Key: "_id", Value: int32(1)},
	}, doc)
}

func BenchmarkTranslatorDocumentSimple(b *testing.B) {
	trans := NewTranslator(&postModel{})

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := trans.Document(bson.M{
			"Title":    "Hello World!",
			"TextBody": "This is awesome.",
		})
		if err != nil {
			panic(err)
		}
	}
}

func BenchmarkTranslatorDocumentComplex(b *testing.B) {
	trans := NewTranslator(&postModel{})

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := trans.Document(bson.M{
			"Title":    "Hello World!",
			"TextBody": []byte("cool"),
		})
		if err != nil {
			panic(err)
		}
	}
}

func BenchmarkTranslatorSort(b *testing.B) {
	trans := NewTranslator(&postModel{})

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := trans.Sort([]string{"Title", "-Text_body"})
		if err != nil {
			panic(err)
		}
	}
}
