package coal

import (
	"context"
	"io"
	"sync"
	"testing"

	"github.com/256dpi/xo"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

func TestConnect(t *testing.T) {
	store := MustConnect("mongodb://0.0.0.0/test-fire-coal", xo.Panic)
	assert.NotNil(t, store.Client)

	assert.Equal(t, "posts", store.C(&postModel{}).Native().Name())

	err := store.Close()
	assert.NoError(t, err)

	assert.Panics(t, func() {
		MustConnect("mongodb://0.0.0.0/test-fire-coal?authMechanism=fail", xo.Panic)
	})
}

func TestOpen(t *testing.T) {
	store := MustOpen(nil, "test-fire-coal", xo.Panic)
	assert.NotNil(t, store.Client)

	assert.Equal(t, "posts", store.C(&postModel{}).Native().Name())

	err := store.Close()
	assert.NoError(t, err)
}

func TestStoreLungo(t *testing.T) {
	assert.True(t, lungoStore.Lungo())
	assert.False(t, mongoStore.Lungo())
}

func TestStoreT(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		assert.False(t, HasTransaction(nil))

		ok, store := GetTransaction(nil)
		assert.False(t, ok)
		assert.Nil(t, store)

		assert.NoError(t, tester.Store.T(nil, false, func(tc context.Context) error {
			assert.True(t, HasTransaction(tc))

			ok, store := GetTransaction(tc)
			assert.True(t, ok)
			assert.Equal(t, tester.Store, store)

			return nil
		}))

		assert.Error(t, tester.Store.T(nil, false, func(tc context.Context) error {
			assert.True(t, HasTransaction(tc))

			ok, store := GetTransaction(tc)
			assert.True(t, ok)
			assert.Equal(t, tester.Store, store)

			return io.EOF
		}))

		tester.Insert(&postModel{})

		assert.Equal(t, 1, tester.Count(&postModel{}))

		assert.NoError(t, tester.Store.T(nil, false, func(tc context.Context) error {
			assert.True(t, HasTransaction(tc))

			ok, store := GetTransaction(tc)
			assert.True(t, ok)
			assert.Equal(t, tester.Store, store)

			_, err := tester.Store.C(&postModel{}).InsertOne(tc, &postModel{
				Base:  B(),
				Title: "foo",
			})
			return err
		}))

		assert.Equal(t, 2, tester.Count(&postModel{}))

		assert.Error(t, tester.Store.T(nil, false, func(tc context.Context) error {
			assert.True(t, HasTransaction(tc))

			ok, store := GetTransaction(tc)
			assert.True(t, ok)
			assert.Equal(t, tester.Store, store)

			_, err := tester.Store.C(&postModel{}).InsertOne(tc, &postModel{
				Base:  B(),
				Title: "bar",
			})
			if err != nil {
				panic(err)
			}

			return io.EOF
		}))

		assert.Equal(t, 2, tester.Count(&postModel{}))

		assert.NoError(t, tester.Store.T(nil, true, func(tc context.Context) error {
			assert.True(t, HasTransaction(tc))

			ok, store := GetTransaction(tc)
			assert.True(t, ok)
			assert.Equal(t, tester.Store, store)

			_, err := tester.Store.C(&postModel{}).DeleteMany(tc, bson.M{})
			if err != nil {
				panic(err)
			}

			return nil
		}))

		assert.Equal(t, 2, tester.Count(&postModel{}))
	})
}

func TestStoreRT(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		if tester.Store.Lungo() {
			return
		}

		post := tester.Insert(&postModel{
			Base:  B(),
			Title: "foo",
		}).(*postModel)

		var attempts int
		var wg sync.WaitGroup
		wg.Add(5)
		for i := 0; i < 5; i++ {
			go func() {
				err := tester.Store.RT(nil, 5, func(ctx context.Context) error {
					attempts++
					var p postModel
					_, err := tester.Store.M(&p).Find(ctx, &p, post.ID(), false)
					if err != nil {
						return err
					}
					_, err = tester.Store.M(post).Update(ctx, post, post.ID(), bson.M{
						"$set": bson.M{
							"Title": p.Title + "-bar",
						},
					}, false)
					if err != nil {
						return err
					}
					return nil
				})
				assert.NoError(t, err)
				wg.Done()
			}()
		}
		wg.Wait()
		assert.True(t, attempts > 5)

		tester.Refresh(post)
		assert.Equal(t, "foo-bar-bar-bar-bar-bar", post.Title)
	})
}
