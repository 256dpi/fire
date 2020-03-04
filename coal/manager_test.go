package coal

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

func TestManagerFind(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		post1 := *tester.Insert(&postModel{
			Title: "Hello World!",
		}).(*postModel)

		m := tester.Store.M(&postModel{})

		// existing
		var post2 postModel
		found, err := m.Find(nil, &post2, post1.ID(), false)
		assert.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, post1, post2)

		// missing
		found, err = m.Find(nil, &post2, New(), false)
		assert.NoError(t, err)
		assert.False(t, found)

		// unexpected lock
		found, err = m.Find(nil, &post2, post1.ID(), true)
		assert.Error(t, err)
		assert.False(t, found)
		assert.Equal(t, ErrUnexpectedLock, err)

		// lock
		_ = tester.Store.T(nil, func(ctx context.Context) error {
			post1.Lock++
			found, err = m.Find(ctx, &post2, post1.ID(), true)
			assert.NoError(t, err)
			assert.True(t, found)
			assert.Equal(t, post1, post2)
			return nil
		})
	})
}

func TestManagerFindFirst(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		post1 := *tester.Insert(&postModel{
			Title: "Hello World!",
		}).(*postModel)

		m := tester.Store.M(&postModel{})

		// existing
		var post2 postModel
		found, err := m.FindFirst(nil, &post2, bson.M{
			"Title": "Hello World!",
		}, nil, 0, false)
		assert.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, post1, post2)

		// missing
		found, err = m.FindFirst(nil, &post2, bson.M{
			"Title": "Foo",
		}, nil, 0, false)
		assert.NoError(t, err)
		assert.False(t, found)

		// unexpected lock
		found, err = m.FindFirst(nil, &post2, bson.M{
			"Title": "Hello World!",
		}, nil, 0, true)
		assert.Error(t, err)
		assert.False(t, found)
		assert.Equal(t, ErrUnexpectedLock, err)

		// lock
		_ = tester.Store.T(nil, func(ctx context.Context) error {
			post1.Lock++
			found, err = m.FindFirst(ctx, &post2, bson.M{
				"Title": "Hello World!",
			}, nil, 0, true)
			assert.NoError(t, err)
			assert.True(t, found)
			assert.Equal(t, post1, post2)
			return nil
		})
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
		}, nil, 0, false)
		if err != nil {
			panic(err)
		} else if !found {
			panic("missing")
		}
	}
}
