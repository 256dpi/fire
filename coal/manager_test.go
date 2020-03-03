package coal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

func TestManager(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		m := tester.Store.M(&postModel{})

		post1 := postModel{
			Title:    "Hello World!",
			TextBody: "This is awesome.",
		}
		err := m.Insert(nil, &post1)
		assert.NoError(t, err)

		var post2 postModel
		found, err := m.FindFirst(nil, &post2, bson.M{
			"Title": "Hello World!",
		}, nil, 0)
		assert.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, post1, post2)
	})
}

func BenchmarkManagerFind(b *testing.B) {
	m := lungoStore.M(&postModel{})

	post1 := &postModel{
		Title:    "Hello World!",
		TextBody: "This is awesome.",
	}

	err := m.Insert(nil, post1)
	if err != nil {
		panic(err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		m := lungoStore.M(&postModel{})

		var post postModel
		found, err := m.FindFirst(nil, &post, bson.M{
			"Title": "Hello World!",
		}, nil, 0)
		if err != nil {
			panic(err)
		} else if !found {
			panic("missing")
		}
	}
}
