package coal

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

func TestFlags(t *testing.T) {
	var f Flags
	assert.False(t, f.Has(NoTransaction))

	f = NoTransaction
	assert.True(t, f.Has(NoTransaction))
}

func TestManagerFind(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		post1 := *tester.Insert(&postModel{
			Title: "Hello World!",
		}).(*postModel)

		m := tester.Store.M(&postModel{})

		// existing
		found, err := m.Find(nil, nil, post1.ID(), false)
		assert.NoError(t, err)
		assert.True(t, found)

		// fetch
		var post2 postModel
		found, err = m.Find(nil, &post2, post1.ID(), false)
		assert.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, post1, post2)

		// missing
		found, err = m.Find(nil, &post2, New(), false)
		assert.NoError(t, err)
		assert.False(t, found)

		// error
		found, err = m.Find(nil, &post2, post1.ID(), true)
		assert.Error(t, err)
		assert.False(t, found)
		assert.True(t, ErrTransactionRequired.Is(err))

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

		post2 := *tester.Insert(&postModel{
			Title: "Hello World!!!",
		}).(*postModel)

		m := tester.Store.M(&postModel{})

		// existing
		found, err := m.FindFirst(nil, nil, bson.M{
			"Title": "Hello World!",
		}, nil, 0, false)
		assert.NoError(t, err)
		assert.True(t, found)

		// fetch
		var post postModel
		found, err = m.FindFirst(nil, &post, bson.M{
			"Title": "Hello World!",
		}, nil, 0, false)
		assert.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, post1, post)

		// sort
		found, err = m.FindFirst(nil, &post, bson.M{
			"Title": "Hello World!!!",
		}, []string{"-Title"}, 0, false)
		assert.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, post2, post)

		// missing
		found, err = m.FindFirst(nil, &post, bson.M{
			"Title": "Hello Space!",
		}, nil, 0, false)
		assert.NoError(t, err)
		assert.False(t, found)

		// error
		found, err = m.FindFirst(nil, &post, bson.M{
			"Title": "Hello World!",
		}, nil, 0, true)
		assert.Error(t, err)
		assert.False(t, found)
		assert.True(t, ErrTransactionRequired.Is(err))

		// lock
		_ = tester.Store.T(nil, func(ctx context.Context) error {
			// fetch
			post1.Lock++
			var post postModel
			found, err = m.FindFirst(ctx, &post, bson.M{
				"Title": "Hello World!",
			}, nil, 0, true)
			assert.NoError(t, err)
			assert.True(t, found)
			assert.Equal(t, post1, post)

			// sort
			post2.Lock++
			found, err = m.FindFirst(nil, &post, bson.M{
				"Title": "Hello World!!!",
			}, []string{"-Title"}, 0, false)
			assert.NoError(t, err)
			assert.True(t, found)
			assert.Equal(t, post2, post)

			return nil
		})
	})
}

func TestManagerFindAll(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		post1 := *tester.Insert(&postModel{
			Title: "Hello World!",
		}).(*postModel)

		post2 := *tester.Insert(&postModel{
			Title: "Hello Space!",
		}).(*postModel)

		m := tester.Store.M(&postModel{})

		// error
		var list []postModel
		err := m.FindAll(nil, &list, nil, nil, 0, 0, false)
		assert.Error(t, err)
		assert.True(t, ErrTransactionRequired.Is(err))

		// unsafe
		err = m.FindAll(nil, &list, nil, nil, 0, 0, false, NoTransaction)
		assert.NoError(t, err)
		assert.Equal(t, []postModel{post1, post2}, list)

		// all
		_ = tester.Store.T(nil, func(ctx context.Context) error {
			var list []postModel
			err := m.FindAll(ctx, &list, nil, nil, 0, 0, false)
			assert.NoError(t, err)
			assert.Equal(t, []postModel{post1, post2}, list)
			return nil
		})

		// filter
		_ = tester.Store.T(nil, func(ctx context.Context) error {
			var list []postModel
			err := m.FindAll(ctx, &list, bson.M{
				"Title": "Hello World!",
			}, nil, 0, 0, false)
			assert.NoError(t, err)
			assert.Equal(t, []postModel{post1}, list)
			return nil
		})

		// sort
		_ = tester.Store.T(nil, func(ctx context.Context) error {
			var list []postModel
			err := m.FindAll(ctx, &list, nil, []string{"Title"}, 0, 0, false)
			assert.NoError(t, err)
			assert.Equal(t, []postModel{post2, post1}, list)
			return nil
		})

		// skip
		_ = tester.Store.T(nil, func(ctx context.Context) error {
			var list []postModel
			err := m.FindAll(ctx, &list, nil, nil, 1, 0, false)
			assert.NoError(t, err)
			assert.Equal(t, []postModel{post2}, list)
			return nil
		})

		// limit
		_ = tester.Store.T(nil, func(ctx context.Context) error {
			var list []postModel
			err := m.FindAll(ctx, &list, nil, nil, 0, 1, false)
			assert.NoError(t, err)
			assert.Equal(t, []postModel{post1}, list)
			return nil
		})

		// lock
		_ = tester.Store.T(nil, func(ctx context.Context) error {
			post1.Lock++
			post2.Lock++

			var list []postModel
			err := m.FindAll(ctx, &list, nil, nil, 0, 0, true)
			assert.NoError(t, err)
			assert.Equal(t, []postModel{post1, post2}, list)

			return nil
		})
	})
}

func TestManagerFindEach(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		post1 := *tester.Insert(&postModel{
			Title: "Hello World!",
		}).(*postModel)

		post2 := *tester.Insert(&postModel{
			Title: "Hello Space!",
		}).(*postModel)

		m := tester.Store.M(&postModel{})

		// error
		iter, err := m.FindEach(nil, nil, nil, 0, 0, false)
		assert.Error(t, err)
		assert.Nil(t, iter)
		assert.True(t, ErrTransactionRequired.Is(err))

		// unsafe
		iter, err = m.FindEach(nil, nil, nil, 0, 0, false, NoTransaction)
		assert.NoError(t, err)
		assert.Equal(t, []postModel{post1, post2}, readPosts(t, iter))

		// all
		_ = tester.Store.T(nil, func(ctx context.Context) error {
			iter, err = m.FindEach(ctx, nil, nil, 0, 0, false)
			assert.NoError(t, err)
			assert.Equal(t, []postModel{post1, post2}, readPosts(t, iter))
			return nil
		})

		// filter
		_ = tester.Store.T(nil, func(ctx context.Context) error {
			iter, err = m.FindEach(ctx, bson.M{
				"Title": "Hello World!",
			}, nil, 0, 0, false)
			assert.NoError(t, err)
			assert.Equal(t, []postModel{post1}, readPosts(t, iter))
			return nil
		})

		// sort
		_ = tester.Store.T(nil, func(ctx context.Context) error {
			iter, err = m.FindEach(ctx, nil, []string{"Title"}, 0, 0, false)
			assert.NoError(t, err)
			assert.Equal(t, []postModel{post2, post1}, readPosts(t, iter))
			return nil
		})

		// skip
		_ = tester.Store.T(nil, func(ctx context.Context) error {
			iter, err = m.FindEach(ctx, nil, nil, 1, 0, false)
			assert.NoError(t, err)
			assert.Equal(t, []postModel{post2}, readPosts(t, iter))
			return nil
		})

		// limit
		_ = tester.Store.T(nil, func(ctx context.Context) error {
			iter, err = m.FindEach(ctx, nil, nil, 0, 1, false)
			assert.NoError(t, err)
			assert.Equal(t, []postModel{post1}, readPosts(t, iter))
			return nil
		})

		// lock
		_ = tester.Store.T(nil, func(ctx context.Context) error {
			post1.Lock++
			post2.Lock++

			iter, err = m.FindEach(ctx, nil, nil, 0, 0, true)
			assert.NoError(t, err)
			assert.Equal(t, []postModel{post1, post2}, readPosts(t, iter))

			return nil
		})
	})
}

func TestManagerCount(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		post1 := *tester.Insert(&postModel{
			Title: "Hello World!",
		}).(*postModel)

		post2 := *tester.Insert(&postModel{
			Title: "Hello Space!",
		}).(*postModel)

		m := tester.Store.M(&postModel{})

		// error
		count, err := m.Count(nil, nil, 0, 0, false)
		assert.Error(t, err)
		assert.Equal(t, int64(0), count)
		assert.True(t, ErrTransactionRequired.Is(err))

		// unsafe
		count, err = m.Count(nil, nil, 0, 0, false, NoTransaction)
		assert.NoError(t, err)
		assert.Equal(t, int64(2), count)

		// all
		_ = tester.Store.T(nil, func(ctx context.Context) error {
			count, err = m.Count(ctx, nil, 0, 0, false)
			assert.NoError(t, err)
			assert.Equal(t, int64(2), count)
			return nil
		})

		// filter
		_ = tester.Store.T(nil, func(ctx context.Context) error {
			count, err = m.Count(ctx, bson.M{
				"Title": "Hello World!",
			}, 0, 0, false)
			assert.NoError(t, err)
			assert.Equal(t, int64(1), count)
			return nil
		})

		// skip
		_ = tester.Store.T(nil, func(ctx context.Context) error {
			count, err = m.Count(ctx, nil, 1, 0, false)
			assert.NoError(t, err)
			assert.Equal(t, int64(1), count)
			return nil
		})

		// limit
		_ = tester.Store.T(nil, func(ctx context.Context) error {
			count, err = m.Count(ctx, nil, 0, 1, false)
			assert.NoError(t, err)
			assert.Equal(t, int64(1), count)
			return nil
		})

		// lock
		_ = tester.Store.T(nil, func(ctx context.Context) error {
			post1.Lock++
			post2.Lock++

			count, err = m.Count(ctx, nil, 0, 0, true)
			assert.NoError(t, err)
			assert.Equal(t, int64(2), count)

			var list []postModel
			err = m.FindAll(ctx, &list, nil, nil, 0, 0, false)
			assert.NoError(t, err)
			assert.Equal(t, []postModel{post1, post2}, list)

			return nil
		})
	})
}

func TestManagerDistinct(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		post1 := *tester.Insert(&postModel{
			Title: "Hello World!",
		}).(*postModel)

		post2 := *tester.Insert(&postModel{
			Title: "Hello Space!",
		}).(*postModel)

		post3 := *tester.Insert(&postModel{
			Title: "Hello Space!",
		}).(*postModel)

		m := tester.Store.M(&postModel{})

		// error
		titles, err := m.Distinct(nil, "Title", nil, false)
		assert.Error(t, err)
		assert.True(t, ErrTransactionRequired.Is(err))

		// unsafe
		titles, err = m.Distinct(nil, "Title", nil, false, NoTransaction)
		assert.NoError(t, err)
		assert.ElementsMatch(t, []interface{}{post1.Title, post2.Title}, titles)

		// all
		_ = tester.Store.T(nil, func(ctx context.Context) error {
			titles, err = m.Distinct(ctx, "Title", nil, false)
			assert.NoError(t, err)
			assert.ElementsMatch(t, []interface{}{post1.Title, post2.Title}, titles)
			return nil
		})

		// filter
		_ = tester.Store.T(nil, func(ctx context.Context) error {
			titles, err = m.Distinct(ctx, "Title", bson.M{
				"Title": "Hello World!",
			}, false)
			assert.NoError(t, err)
			assert.ElementsMatch(t, []interface{}{post1.Title}, titles)
			return nil
		})

		// lock
		_ = tester.Store.T(nil, func(ctx context.Context) error {
			post1.Lock++
			post2.Lock++
			post3.Lock++

			titles, err := m.Distinct(ctx, "Title", nil, true)
			assert.NoError(t, err)
			assert.ElementsMatch(t, []interface{}{post1.Title, post2.Title}, titles)

			var list []postModel
			err = m.FindAll(ctx, &list, nil, nil, 0, 0, false)
			assert.NoError(t, err)
			assert.Equal(t, []postModel{post1, post2, post3}, list)

			return nil
		})
	})
}

func TestManagerInsert(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		m := tester.Store.M(&postModel{})

		err := m.Insert(nil, &postModel{
			Title: "Hello World!",
		})
		assert.NoError(t, err)
		assert.Equal(t, 1, tester.Count(&postModel{}))
	})
}

func TestManagerInsertIfMissing(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		m := tester.Store.M(&postModel{})

		// insert if missing
		inserted, err := m.InsertIfMissing(nil, bson.M{
			"Title": "Hello World!",
		}, &postModel{
			Title: "Hello World!",
		}, false)
		assert.NoError(t, err)
		assert.True(t, inserted)

		// insert if missing again
		inserted, err = m.InsertIfMissing(nil, bson.M{
			"Title": "Hello World!",
		}, &postModel{
			Title: "Hello World!",
		}, false)
		assert.NoError(t, err)
		assert.False(t, inserted)

		// error
		inserted, err = m.InsertIfMissing(nil, bson.M{
			"Title": "Hello World!",
		}, &postModel{
			Title: "Hello World!",
		}, true)
		assert.Error(t, err)
		assert.False(t, inserted)
		assert.True(t, ErrTransactionRequired.Is(err))

		// lock
		_ = tester.Store.T(nil, func(ctx context.Context) error {
			inserted, err = m.InsertIfMissing(ctx, bson.M{
				"Title": "Hello World!",
			}, &postModel{
				Title: "Hello World!",
			}, true)
			assert.NoError(t, err)
			assert.False(t, inserted)

			return nil
		})
	})
}

func TestManagerReplace(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		post := tester.Insert(&postModel{
			Title: "Hello World!",
		}).(*postModel)

		m := tester.Store.M(&postModel{})

		// missing
		found, err := m.Replace(nil, &postModel{
			Base: B(),
		}, false)
		assert.NoError(t, err)
		assert.False(t, found)

		// existing
		post.Title = "Hello Space!"
		found, err = m.Replace(nil, post, false)
		assert.NoError(t, err)
		assert.True(t, found)

		// error
		found, err = m.Replace(nil, post, true)
		assert.Error(t, err)
		assert.False(t, found)
		assert.True(t, ErrTransactionRequired.Is(err))

		// lock
		_ = tester.Store.T(nil, func(ctx context.Context) error {
			post.Title = "Hello Space!"
			found, err = m.Replace(ctx, post, true)
			assert.NoError(t, err)
			assert.True(t, found)

			return nil
		})
	})
}

func TestManagerReplaceFirst(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		post := tester.Insert(&postModel{
			Title: "Hello World!",
		}).(*postModel)

		m := tester.Store.M(&postModel{})

		// missing
		found, err := m.ReplaceFirst(nil, bson.M{
			"Title": "Hello Space!",
		}, &postModel{
			Base: B(),
		}, false)
		assert.NoError(t, err)
		assert.False(t, found)

		// existing
		post.Title = "Hello Space!"
		found, err = m.ReplaceFirst(nil, bson.M{
			"Title": "Hello World!",
		}, post, false)
		assert.NoError(t, err)
		assert.True(t, found)

		// error
		found, err = m.ReplaceFirst(nil, bson.M{
			"Title": "Hello World!",
		}, post, true)
		assert.Error(t, err)
		assert.False(t, found)
		assert.True(t, ErrTransactionRequired.Is(err))

		// lock
		_ = tester.Store.T(nil, func(ctx context.Context) error {
			post.Title = "Hello World!"
			found, err = m.ReplaceFirst(ctx, bson.M{
				"Title": "Hello Space!",
			}, post, true)
			assert.NoError(t, err)
			assert.True(t, found)

			return nil
		})
	})
}

func TestManagerUpdate(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		post := tester.Insert(&postModel{
			Title: "Hello World!",
		}).(*postModel)

		m := tester.Store.M(&postModel{})

		// missing
		found, err := m.Update(nil, nil, New(), bson.M{
			"$set": bson.M{
				"Title": "Hello World!!!",
			},
		}, false)
		assert.NoError(t, err)
		assert.False(t, found)

		// missing
		var updated postModel
		found, err = m.Update(nil, &updated, New(), bson.M{
			"$set": bson.M{
				"Title": "Hello World!!!",
			},
		}, false)
		assert.NoError(t, err)
		assert.False(t, found)

		// existing
		found, err = m.Update(nil, nil, post.ID(), bson.M{
			"$set": bson.M{
				"Title": "Hello World!!!",
			},
		}, false)
		assert.NoError(t, err)
		assert.True(t, found)

		// existing
		found, err = m.Update(nil, &updated, post.ID(), bson.M{
			"$set": bson.M{
				"Title": "Hello Space!",
			},
		}, false)
		assert.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, postModel{
			Base:  post.Base,
			Title: "Hello Space!",
		}, updated)

		// error
		found, err = m.Update(nil, nil, post.ID(), bson.M{
			"$set": bson.M{
				"Title": "Hello Space!",
			},
		}, true)
		assert.Error(t, err)
		assert.False(t, found)
		assert.True(t, ErrTransactionRequired.Is(err))

		// lock
		_ = tester.Store.T(nil, func(ctx context.Context) error {
			post.Lock++

			found, err = m.Update(ctx, &updated, post.ID(), bson.M{
				"$set": bson.M{
					"Title": "Hello Space!!!",
				},
			}, true)
			assert.NoError(t, err)
			assert.True(t, found)
			assert.Equal(t, postModel{
				Base:  post.Base,
				Title: "Hello Space!!!",
			}, updated)

			return nil
		})
	})
}

func TestManagerUpdateFirst(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		post := *tester.Insert(&postModel{
			Title: "Hello World!",
		}).(*postModel)

		m := tester.Store.M(&postModel{})

		// missing
		found, err := m.UpdateFirst(nil, nil, bson.M{
			"Title": "Hello Space!",
		}, bson.M{
			"$set": bson.M{
				"Title": "Hello World!",
			},
		}, nil, false)
		assert.NoError(t, err)
		assert.False(t, found)

		// missing
		var updated postModel
		found, err = m.UpdateFirst(nil, &updated, bson.M{
			"Title": "Hello Space!",
		}, bson.M{
			"$set": bson.M{
				"Title": "Hello World!",
			},
		}, nil, false)
		assert.NoError(t, err)
		assert.False(t, found)

		// existing
		found, err = m.UpdateFirst(nil, nil, bson.M{
			"Title": "Hello World!",
		}, bson.M{
			"$set": bson.M{
				"Title": "Hello Space!",
			},
		}, nil, false)
		assert.NoError(t, err)
		assert.True(t, found)

		// existing
		found, err = m.UpdateFirst(nil, &updated, bson.M{
			"Title": "Hello Space!",
		}, bson.M{
			"$set": bson.M{
				"Title": "Hello World!",
			},
		}, nil, false)
		assert.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, postModel{
			Base:  post.Base,
			Title: "Hello World!",
		}, updated)

		// error
		found, err = m.UpdateFirst(nil, nil, bson.M{
			"Title": "Hello Space!",
		}, bson.M{
			"$set": bson.M{
				"Title": "Hello World!",
			},
		}, nil, true)
		assert.Error(t, err)
		assert.False(t, found)
		assert.True(t, ErrTransactionRequired.Is(err))

		// lock
		_ = tester.Store.T(nil, func(ctx context.Context) error {
			post.Lock++

			found, err = m.UpdateFirst(ctx, &updated, bson.M{
				"Title": "Hello World!",
			}, bson.M{
				"$set": bson.M{
					"Title": "Hello Space!",
				},
			}, nil, true)
			assert.NoError(t, err)
			assert.True(t, found)
			assert.Equal(t, postModel{
				Base:  post.Base,
				Title: "Hello Space!",
			}, updated)

			return nil
		})
	})
}

func TestManagerUpdateAll(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		tester.Insert(&postModel{
			Title: "Hello World!",
		})
		tester.Insert(&postModel{
			Title: "Hello World!",
		})

		m := tester.Store.M(&postModel{})

		// missing
		matched, err := m.UpdateAll(nil, bson.M{
			"Title": "foo",
		}, bson.M{
			"$set": bson.M{
				"Title": "Hello Space!",
			},
		}, false)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), matched)

		// existing
		matched, err = m.UpdateAll(nil, bson.M{
			"Title": "Hello World!",
		}, bson.M{
			"$set": bson.M{
				"Title": "Hello Space!",
			},
		}, false)
		assert.NoError(t, err)
		assert.Equal(t, int64(2), matched)

		// error
		matched, err = m.UpdateAll(nil, bson.M{
			"Title": "Hello Space!",
		}, bson.M{
			"$set": bson.M{
				"Title": "Hello World!",
			},
		}, true)
		assert.Error(t, err)
		assert.Equal(t, int64(0), matched)
		assert.True(t, ErrTransactionRequired.Is(err))

		// lock
		_ = tester.Store.T(nil, func(ctx context.Context) error {
			matched, err = m.UpdateAll(ctx, bson.M{
				"Title": "Hello Space!",
			}, bson.M{
				"$set": bson.M{
					"Title": "Hello World!",
				},
			}, true)
			assert.NoError(t, err)
			assert.Equal(t, int64(2), matched)

			return nil
		})
	})
}

func TestManagerUpsert(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		m := tester.Store.M(&postModel{})

		// upsert (insert missing)
		inserted, err := m.Upsert(nil, nil, bson.M{
			"Title": "Hello World!",
		}, bson.M{
			"$set": bson.M{
				"Title": "Hello World!!!",
			},
		}, nil, false)
		assert.NoError(t, err)
		assert.True(t, inserted)

		// upsert again (update existing)
		inserted, err = m.Upsert(nil, nil, bson.M{
			"Title": "Hello World!!!",
		}, bson.M{
			"$set": bson.M{
				"Published": true,
			},
		}, nil, false)
		assert.NoError(t, err)
		assert.False(t, inserted)

		// upsert (insert missing)
		var post postModel
		inserted, err = m.Upsert(nil, &post, bson.M{
			"Title": "Hello Space!",
		}, bson.M{
			"$set": bson.M{
				"Title": "Hello Space!!!",
			},
		}, nil, false)
		assert.NoError(t, err)
		assert.True(t, inserted)
		assert.Equal(t, postModel{
			Base:  post.Base,
			Title: "Hello Space!!!",
		}, post)

		// upsert again (update existing)
		inserted, err = m.Upsert(nil, &post, bson.M{
			"Title": "Hello Space!!!",
		}, bson.M{
			"$set": bson.M{
				"Published": true,
			},
		}, nil, false)
		assert.NoError(t, err)
		assert.False(t, inserted)
		assert.Equal(t, postModel{
			Base:      post.Base,
			Title:     "Hello Space!!!",
			Published: true,
		}, post)

		// error
		inserted, err = m.Upsert(nil, nil, bson.M{
			"Title": "Hello World!",
		}, bson.M{
			"$set": bson.M{
				"Title": "Hello World!",
			},
		}, nil, true)
		assert.Error(t, err)
		assert.False(t, inserted)
		assert.True(t, ErrTransactionRequired.Is(err))

		// lock
		_ = tester.Store.T(nil, func(ctx context.Context) error {
			post.Lock++

			post.Published = false
			var lockedPost postModel
			inserted, err = m.Upsert(ctx, &lockedPost, bson.M{
				"Title": "Hello Space!!!",
			}, bson.M{
				"$set": bson.M{
					"Published": false,
				},
			}, nil, true)
			assert.NoError(t, err)
			assert.False(t, inserted)
			assert.Equal(t, post, lockedPost)

			return nil
		})
	})
}

func TestManagerDelete(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		post := *tester.Insert(&postModel{
			Title: "Hello World!",
		}).(*postModel)

		m := tester.Store.M(&postModel{})

		// missing
		found, err := m.Delete(nil, nil, New())
		assert.NoError(t, err)
		assert.False(t, found)

		// existing
		var deleted postModel
		found, err = m.Delete(nil, &deleted, post.ID())
		assert.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, post, deleted)
	})
}

func TestManagerDeleteAll(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		tester.Insert(&postModel{
			Title: "Hello World!",
		})
		tester.Insert(&postModel{
			Title: "Hello World!",
		})

		m := tester.Store.M(&postModel{})

		// missing
		deleted, err := m.DeleteAll(nil, bson.M{
			"Title": "foo",
		})
		assert.NoError(t, err)
		assert.Equal(t, int64(0), deleted)

		// existing
		deleted, err = m.DeleteAll(nil, bson.M{
			"Title": "Hello World!",
		})
		assert.NoError(t, err)
		assert.Equal(t, int64(2), deleted)
	})
}

func TestManagerDeleteFirst(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		tester.Insert(&postModel{
			Title: "Hello World!",
		})

		post2 := *tester.Insert(&postModel{
			Title: "Hello Space!",
		}).(*postModel)

		m := tester.Store.M(&postModel{})

		// missing
		found, err := m.DeleteFirst(nil, nil, bson.M{
			"Title": "foo",
		}, nil)
		assert.NoError(t, err)
		assert.False(t, found)

		// missing
		var deleted postModel
		found, err = m.DeleteFirst(nil, &deleted, bson.M{
			"Title": "foo",
		}, nil)
		assert.NoError(t, err)
		assert.False(t, found)

		// existing
		found, err = m.DeleteFirst(nil, nil, bson.M{
			"Title": "Hello World!",
		}, nil)
		assert.NoError(t, err)
		assert.True(t, found)

		// existing
		found, err = m.DeleteFirst(nil, &deleted, bson.M{
			"Title": "Hello Space!",
		}, nil)
		assert.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, post2, deleted)
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

func readPosts(t *testing.T, iter *Iterator) []postModel {
	defer iter.Close()
	var list []postModel
	for iter.Next() {
		var post postModel
		err := iter.Decode(&post)
		assert.NoError(t, err)
		list = append(list, post)
	}
	return list
}
