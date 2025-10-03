package coal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/stick"
)

func TestBaseID(t *testing.T) {
	id := New()
	post := &postModel{Base: B(id)}
	assert.Equal(t, id, post.ID())
	assert.Equal(t, id, post.DocID)
	assert.Equal(t, id, post.Base.DocID)
}

func TestDynamicAccess(t *testing.T) {
	post := &postModel{
		Title: "title",
	}

	val, ok := stick.Get(post, "title")
	assert.False(t, ok)
	assert.Nil(t, val)

	val, ok = stick.Get(post, "Title")
	assert.True(t, ok)
	assert.Equal(t, "title", val)

	ok = stick.Set(post, "title", "foo")
	assert.False(t, ok)
	assert.Equal(t, "title", post.Title)

	ok = stick.Set(post, "Title", "foo")
	assert.True(t, ok)
	assert.Equal(t, "foo", post.Title)
}

func TestList(t *testing.T) {
	list := List[*listItem]{{
		Title: "Foo",
		Done:  false,
	}}

	list = append(list, &listItem{
		Title: "Bar",
		Done:  true,
	})

	err := list.Validate()
	assert.NoError(t, err)
	assert.NotEmpty(t, list[0].ItemID)
	assert.NotEmpty(t, list[1].ItemID)

	list[0].ItemID = "a"
	list[1].ItemID = "b"

	var out []stick.Map
	err = stick.BSON.Transfer(list, &out)
	assert.NoError(t, err)
	assert.Equal(t, []stick.Map{
		{
			"_id":   "a",
			"title": "Foo",
			"done":  false,
		}, {
			"_id":   "b",
			"title": "Bar",
			"done":  true,
		},
	}, out)

	out = nil
	err = stick.JSON.Transfer(list, &out)
	assert.NoError(t, err)
	assert.Equal(t, []stick.Map{
		{
			"id":    "a",
			"title": "Foo",
			"done":  false,
		}, {
			"id":    "b",
			"title": "Bar",
			"done":  true,
		},
	}, out)

	/* move items */

	out[0], out[1] = out[1], out[0]
	out = append(out, stick.Map{
		"id":    "c",
		"title": "Baz",
		"done":  false,
	})

	err = stick.JSON.Transfer(out, &list)
	assert.NoError(t, err)
	assert.Equal(t, List[*listItem]{
		{
			ItemBase: I("b"),
			Title:    "Bar",
			Done:     true,
		},
		{
			ItemBase: I("a"),
			Title:    "Foo",
			Done:     false,
		},
		{
			ItemBase: I("c"),
			Title:    "Baz",
			Done:     false,
		},
	}, list)

	list = append(list, nil)
	err = list.Validate()
	assert.ErrorContains(t, err, "nil item")
}

func TestSlice(t *testing.T) {
	list1 := []postModel{{Title: "foo"}}
	slice1a := Slice(list1)
	slice1b := Slice(&list1)
	list1[0].Title = "bar"
	assert.Equal(t, []Model{&postModel{Title: "bar"}}, slice1a)
	assert.Equal(t, []Model{&postModel{Title: "bar"}}, slice1b)

	list2 := []*postModel{{Title: "foo"}}
	slice2a := Slice(list2)
	slice2b := Slice(&list2)
	list2[0].Title = "bar"
	assert.Equal(t, []Model{&postModel{Title: "bar"}}, slice2a)
	assert.Equal(t, []Model{&postModel{Title: "bar"}}, slice2b)
}

func TestRegistry(t *testing.T) {
	registry := NewRegistry(&postModel{}, &commentModel{})
	assert.Equal(t, []Model{&postModel{}, &commentModel{}}, registry.All())

	registry.Add(&noteModel{})
	assert.Equal(t, []Model{&postModel{}, &commentModel{}, &noteModel{}}, registry.All())

	model := registry.Lookup("posts")
	assert.Equal(t, &postModel{}, model)

	model = registry.Lookup("foo")
	assert.Nil(t, model)
}

func TestTags(t *testing.T) {
	post := &postModel{}

	value := post.GetTag("foo")
	assert.Nil(t, value)

	post.SetTag("foo", "bar", time.Now().Add(time.Minute))

	value = post.GetTag("foo")
	assert.Equal(t, value, "bar")

	post.SetTag("foo", nil, time.Time{})

	value = post.GetTag("foo")
	assert.Nil(t, value)

	withTester(t, func(t *testing.T, tester *Tester) {
		post := tester.Insert(&postModel{})

		/* missing */

		value := post.GetBase().GetTag("foo")
		assert.Nil(t, value)

		n := tester.Count(&postModel{}, bson.M{
			TV("foo"): "bar",
		})
		assert.Equal(t, 0, n)

		/* no expiry */

		post = tester.Update(post, bson.M{
			"$set": bson.M{
				TV("foo"): "bar",
			},
		})

		value = post.GetBase().GetTag("foo")
		assert.Equal(t, value, "bar")

		n = tester.Count(&postModel{}, bson.M{
			TV("foo"): "bar",
		})
		assert.Equal(t, 1, n)

		n = tester.Count(&postModel{}, bson.M{
			TV("foo"): "bar",
			TE("foo"): TF(true),
		})
		assert.Equal(t, 0, n)

		n = tester.Count(&postModel{}, bson.M{
			TV("foo"): "bar",
			TE("foo"): TF(false),
		})
		assert.Equal(t, 1, n)

		/* valid */

		post = tester.Update(post, bson.M{
			"$set": bson.M{
				TE("foo"): time.Now().Add(time.Minute),
			},
		})

		value = post.GetBase().GetTag("foo")
		assert.Equal(t, value, "bar")

		n = tester.Count(&postModel{}, bson.M{
			TV("foo"): "bar",
		})
		assert.Equal(t, 1, n)

		n = tester.Count(&postModel{}, bson.M{
			TV("foo"): "bar",
			TE("foo"): TF(true),
		})
		assert.Equal(t, 0, n)

		n = tester.Count(&postModel{}, bson.M{
			TV("foo"): "bar",
			TE("foo"): TF(false),
		})
		assert.Equal(t, 1, n)

		/* expired */

		post = tester.Update(post, bson.M{
			"$set": bson.M{
				T("foo"): Tag{
					Value:  "baz",
					Expiry: time.Now().Add(-time.Minute),
				},
			},
		})

		value = post.GetBase().GetTag("foo")
		assert.Nil(t, value)

		n = tester.Count(&postModel{}, bson.M{
			TV("foo"): "baz",
		})
		assert.Equal(t, 1, n)

		n = tester.Count(&postModel{}, bson.M{
			TE("foo"): TF(true),
		})
		assert.Equal(t, 1, n)

		n = tester.Count(&postModel{}, bson.M{
			TE("foo"): TF(false),
		})
		assert.Equal(t, 0, n)

		/* missing */

		post = tester.Update(post, bson.M{
			"$unset": bson.M{
				T("foo"): "",
			},
		})

		value = post.GetBase().GetTag("foo")
		assert.Nil(t, value)

		n = tester.Count(&postModel{}, bson.M{
			T("foo"): "bar",
		})
		assert.Equal(t, 0, n)
	})
}

func TestClean(t *testing.T) {
	post := &postModel{}

	value := post.GetTag("foo")
	assert.Nil(t, value)

	expiry := time.Now().Add(time.Minute)
	post.SetTag("foo", "bar", expiry)

	Clean(post)
	assert.Equal(t, map[string]Tag{
		"foo": {
			Value:  "bar",
			Expiry: expiry,
		},
	}, post.Base.Tags)

	post.SetTag("foo", "bar", time.Now().Add(-time.Minute))

	Clean(post)
	assert.Equal(t, map[string]Tag{}, post.Base.Tags)
}
