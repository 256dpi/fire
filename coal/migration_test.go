package coal

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/256dpi/xo"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/stick"
)

type fooModel struct {
	Base               `json:"-" bson:",inline" coal:"foos"`
	Name               string     `json:"name" bson:",omitempty"`
	Body               string     `json:"body" bson:",omitempty"`
	Foos               []fooModel `json:"foos"`
	stick.NoValidation `json:"-" bson:"-"`
}

func TestMigrator(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		/* logging */

		xo.Test(func(xt *xo.Tester) {
			m := NewMigrator()
			m.Add(Migration{
				Name: "foo",
				Migrator: func(ctx context.Context, store *Store) (int64, int64, error) {
					return 2, 1, nil
				},
			})

			err := m.Run(tester.Store, xo.Sink("MIGRATOR"), nil)
			assert.NoError(t, err)

			assert.Equal(t, []string{
				"running migration: foo",
				"completed migration: 2 matched, 1 modified",
			}, strings.Split(strings.TrimSpace(xt.Sinks["MIGRATOR"].String), "\n"))
		})

		/* synchronous error */

		m := NewMigrator()
		m.Add(Migration{
			Name: "bar",
			Migrator: func(ctx context.Context, store *Store) (int64, int64, error) {
				return 0, 0, errors.New("error")
			},
		})

		err := m.Run(tester.Store, nil, nil)
		assert.Error(t, err)
		assert.Equal(t, "error", err.Error())

		/* asynchronous error */

		m = NewMigrator()
		m.Add(Migration{
			Name:  "bar",
			Async: true,
			Migrator: func(ctx context.Context, store *Store) (int64, int64, error) {
				time.Sleep(10 * time.Millisecond)
				return 0, 0, errors.New("error")
			},
		})

		var asyncError error
		err = m.Run(tester.Store, nil, func(err error) {
			asyncError = err
		})
		assert.NoError(t, err)
		assert.NoError(t, asyncError)

		time.Sleep(100 * time.Millisecond)
		assert.Error(t, asyncError)
		assert.Equal(t, "error", asyncError.Error())
	})
}

func TestProcessEach(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		for i := 0; i < 20; i++ {
			tester.Insert(&fooModel{
				Name: "foo-" + strconv.Itoa(i),
			})
		}

		matched, modified, err := ProcessEach(nil, tester.Store, &fooModel{}, bson.M{}, 1, func(model Model) error {
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, int64(20), matched)
		assert.Equal(t, int64(20), modified)

		matched, modified, err = ProcessEach(nil, tester.Store, &fooModel{}, bson.M{}, 5, func(model Model) error {
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, int64(20), matched)
		assert.Equal(t, int64(20), modified)

		matched, modified, err = ProcessEach(nil, tester.Store, &fooModel{}, bson.M{}, 1, func(model Model) error {
			if model.(*fooModel).Name == "foo-3" {
				return errors.New("foo")
			}
			return nil
		})
		assert.Error(t, err)
		assert.Equal(t, int64(3), matched)
		assert.Equal(t, int64(3), modified)

		_, _, err = ProcessEach(nil, tester.Store, &fooModel{}, bson.M{}, 5, func(model Model) error {
			if model.(*fooModel).Name == "foo-3" {
				return errors.New("foo")
			}
			return nil
		})
		assert.Error(t, err)
	})
}

func TestFindEachAndReplace(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		tester.Insert(&fooModel{
			Name: "foo",
		})

		tester.Insert(&fooModel{
			Name: "bar",
		})

		matched, modified, err := FindEachAndReplace(nil, tester.Store, &fooModel{}, bson.M{
			"Name": "foo",
		}, 1, func(model Model) error {
			stick.Set(model, "Body", "baz")
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, int64(1), matched)
		assert.Equal(t, int64(1), modified)

		foos := *tester.FindAll(&fooModel{}).(*[]*fooModel)
		assert.Equal(t, []*fooModel{
			{Base: foos[0].Base, Name: "foo", Body: "baz"},
			{Base: foos[1].Base, Name: "bar"},
		}, foos)
	})
}

func TestFindEachAndUpdate(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		tester.Insert(&fooModel{
			Name: "foo",
		})

		tester.Insert(&fooModel{
			Name: "bar",
		})

		matched, modified, err := FindEachAndUpdate(nil, tester.Store, &fooModel{}, bson.M{
			"Name": "foo",
		}, 1, func(model Model) (bson.M, error) {
			return bson.M{
				"$set": bson.M{
					"Body": "baz",
				},
			}, nil
		})
		assert.NoError(t, err)
		assert.Equal(t, int64(1), matched)
		assert.Equal(t, int64(1), modified)

		foos := *tester.FindAll(&fooModel{}).(*[]*fooModel)
		assert.Equal(t, []*fooModel{
			{Base: foos[0].Base, Name: "foo", Body: "baz"},
			{Base: foos[1].Base, Name: "bar"},
		}, foos)
	})
}

func TestEnsureField(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		tester.Insert(&fooModel{
			Name: "foo",
		})

		tester.Insert(&fooModel{
			Name: "bar",
			Body: "bar",
		})

		matched, modified, err := EnsureField(nil, tester.Store, &fooModel{}, "body", "baz")
		assert.NoError(t, err)
		assert.Equal(t, int64(1), matched)
		assert.Equal(t, int64(1), modified)

		matched, modified, err = EnsureField(nil, tester.Store, &fooModel{}, "body", "baz")
		assert.NoError(t, err)
		assert.Equal(t, int64(0), matched)
		assert.Equal(t, int64(0), modified)

		foos := *tester.FindAll(&fooModel{}).(*[]*fooModel)
		assert.Equal(t, []*fooModel{
			{Base: foos[0].Base, Name: "foo", Body: "baz"},
			{Base: foos[1].Base, Name: "bar", Body: "bar"},
		}, foos)
	})
}

func TestRenameFields(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		tester.Insert(&fooModel{
			Name: "foo",
		})

		tester.Insert(&fooModel{
			Name: "bar",
			Body: "baz",
		})

		matched, modified, err := RenameFields(nil, tester.Store, &fooModel{}, map[string]string{"body": "name"})
		assert.NoError(t, err)
		assert.Equal(t, int64(1), matched)
		assert.Equal(t, int64(1), modified)

		foos := *tester.FindAll(&fooModel{}).(*[]*fooModel)
		assert.Equal(t, []*fooModel{
			{Base: foos[0].Base, Name: "foo"},
			{Base: foos[1].Base, Name: "baz"},
		}, foos)

		matched, modified, err = RenameFields(nil, tester.Store, &fooModel{}, map[string]string{"name": "_1", "body": "_2"})
		assert.NoError(t, err)
		assert.Equal(t, int64(2), matched)
		assert.Equal(t, int64(2), modified)

		foos = *tester.FindAll(&fooModel{}).(*[]*fooModel)
		assert.Equal(t, []*fooModel{
			{Base: foos[0].Base},
			{Base: foos[1].Base},
		}, foos)

		matched, modified, err = RenameFields(nil, tester.Store, &fooModel{}, map[string]string{"name": "_1", "body": "_2"})
		assert.NoError(t, err)
		assert.Equal(t, int64(0), matched)
		assert.Equal(t, int64(0), modified)
	})
}

func TestUnsetFields(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		tester.Insert(&fooModel{
			Name: "foo",
		})

		tester.Insert(&fooModel{
			Name: "bar",
			Body: "baz",
		})

		matched, modified, err := UnsetFields(nil, tester.Store, &fooModel{}, "name")
		assert.NoError(t, err)
		assert.Equal(t, int64(2), matched)
		assert.Equal(t, int64(2), modified)

		matched, modified, err = UnsetFields(nil, tester.Store, &fooModel{}, "name")
		assert.NoError(t, err)
		assert.Equal(t, int64(0), matched)
		assert.Equal(t, int64(0), modified)

		foos := *tester.FindAll(&fooModel{}).(*[]*fooModel)
		assert.Equal(t, []*fooModel{
			{Base: foos[0].Base},
			{Base: foos[1].Base, Body: "baz"},
		}, foos)

		matched, modified, err = UnsetFields(nil, tester.Store, &fooModel{}, "name", "body")
		assert.NoError(t, err)
		assert.Equal(t, int64(1), matched)
		assert.Equal(t, int64(1), modified)

		matched, modified, err = UnsetFields(nil, tester.Store, &fooModel{}, "name", "body")
		assert.NoError(t, err)
		assert.Equal(t, int64(0), matched)
		assert.Equal(t, int64(0), modified)

		foos = *tester.FindAll(&fooModel{}).(*[]*fooModel)
		assert.Equal(t, []*fooModel{
			{Base: foos[0].Base},
			{Base: foos[1].Base},
		}, foos)
	})
}

func TestEnsureArrayField(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		if tester.Store.Lungo() {
			assert.PanicsWithValue(t, "coal: not supported by lungo", func() {
				_, _, _ = EnsureArrayField(nil, tester.Store, nil, "", "", "")
			})

			return
		}

		tester.Insert(&fooModel{
			Name: "foo",
			Foos: []fooModel{
				{
					Name: "foo",
				},
				{
					Name: "foo",
				},
			},
		})

		tester.Insert(&fooModel{
			Name: "bar",
			Foos: []fooModel{
				{
					Name: "bar",
				},
				{
					Body: "bar",
				},
			},
		})

		matched, modified, err := EnsureArrayField(nil, tester.Store, &fooModel{}, "foos", "body", "baz")
		assert.NoError(t, err)
		assert.Equal(t, int64(2), matched)
		assert.Equal(t, int64(2), modified)

		matched, modified, err = EnsureArrayField(nil, tester.Store, &fooModel{}, "foos", "body", "baz")
		assert.NoError(t, err)
		assert.Equal(t, int64(0), matched)
		assert.Equal(t, int64(0), modified)

		foos := *tester.FindAll(&fooModel{}).(*[]*fooModel)
		assert.Equal(t, []*fooModel{
			{
				Base: foos[0].Base,
				Name: "foo",
				Foos: []fooModel{
					{
						Name: "foo",
						Body: "baz",
					},
					{
						Name: "foo",
						Body: "baz",
					},
				},
			},
			{
				Base: foos[1].Base,
				Name: "bar",
				Foos: []fooModel{
					{
						Name: "bar",
						Body: "baz",
					},
					{
						Body: "bar",
					},
				},
			},
		}, foos)
	})
}

func TestRenameArrayFields(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		if tester.Store.Lungo() {
			assert.PanicsWithValue(t, "coal: not supported by lungo", func() {
				_, _, _ = RenameArrayFields(nil, tester.Store, nil, "", nil)
			})

			return
		}

		tester.Insert(&fooModel{
			Name: "foo",
			Foos: []fooModel{
				{
					Name: "foo",
				},
				{
					Name: "foo",
					Body: "bar",
				},
			},
		})

		tester.Insert(&fooModel{
			Name: "bar",
			Foos: []fooModel{
				{
					Name: "bar",
				},
				{
					Body: "bar",
				},
			},
		})

		matched, modified, err := RenameArrayFields(nil, tester.Store, &fooModel{}, "foos", map[string]string{"name": "body"})
		assert.NoError(t, err)
		assert.Equal(t, int64(2), matched)
		assert.Equal(t, int64(2), modified)

		matched, modified, err = RenameArrayFields(nil, tester.Store, &fooModel{}, "foos", map[string]string{"name": "body"})
		assert.NoError(t, err)
		assert.Equal(t, int64(0), matched)
		assert.Equal(t, int64(0), modified)

		foos := *tester.FindAll(&fooModel{}).(*[]*fooModel)
		assert.Equal(t, []*fooModel{
			{
				Base: foos[0].Base,
				Name: "foo",
				Foos: []fooModel{
					{
						Body: "foo",
					},
					{
						Body: "foo",
					},
				},
			},
			{
				Base: foos[1].Base,
				Name: "bar",
				Foos: []fooModel{
					{
						Body: "bar",
					},
					{
						Body: "bar",
					},
				},
			},
		}, foos)

		matched, modified, err = RenameArrayFields(nil, tester.Store, &fooModel{}, "foos", map[string]string{"name": "_1", "body": "_2"})
		assert.NoError(t, err)
		assert.Equal(t, int64(2), matched)
		assert.Equal(t, int64(2), modified)

		matched, modified, err = RenameArrayFields(nil, tester.Store, &fooModel{}, "foos", map[string]string{"name": "_1", "body": "_2"})
		assert.NoError(t, err)
		assert.Equal(t, int64(0), matched)
		assert.Equal(t, int64(0), modified)

		foos = *tester.FindAll(&fooModel{}).(*[]*fooModel)
		assert.Equal(t, []*fooModel{
			{
				Base: foos[0].Base,
				Name: "foo",
				Foos: []fooModel{
					{},
					{},
				},
			},
			{
				Base: foos[1].Base,
				Name: "bar",
				Foos: []fooModel{
					{},
					{},
				},
			},
		}, foos)
	})
}

func TestUnsetArrayFields(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		if tester.Store.Lungo() {
			assert.PanicsWithValue(t, "coal: not supported by lungo", func() {
				_, _, _ = UnsetArrayFields(nil, tester.Store, nil, "")
			})

			return
		}

		tester.Insert(&fooModel{
			Name: "foo",
			Foos: []fooModel{
				{
					Name: "foo",
				},
				{
					Name: "foo",
				},
			},
		})

		tester.Insert(&fooModel{
			Name: "bar",
			Foos: []fooModel{
				{
					Name: "bar",
				},
				{
					Body: "bar",
				},
			},
		})

		matched, modified, err := UnsetArrayFields(nil, tester.Store, &fooModel{}, "foos", "name")
		assert.NoError(t, err)
		assert.Equal(t, int64(2), matched)
		assert.Equal(t, int64(2), modified)

		matched, modified, err = UnsetArrayFields(nil, tester.Store, &fooModel{}, "foos", "name")
		assert.NoError(t, err)
		assert.Equal(t, int64(0), matched)
		assert.Equal(t, int64(0), modified)

		foos := *tester.FindAll(&fooModel{}).(*[]*fooModel)
		assert.Equal(t, []*fooModel{
			{
				Base: foos[0].Base,
				Name: "foo",
				Foos: []fooModel{
					{},
					{},
				},
			},
			{
				Base: foos[1].Base,
				Name: "bar",
				Foos: []fooModel{
					{},
					{Body: "bar"},
				},
			},
		}, foos)

		matched, modified, err = UnsetArrayFields(nil, tester.Store, &fooModel{}, "foos", "name", "body")
		assert.NoError(t, err)
		assert.Equal(t, int64(1), matched)
		assert.Equal(t, int64(1), modified)

		matched, modified, err = UnsetArrayFields(nil, tester.Store, &fooModel{}, "foos", "name", "body")
		assert.NoError(t, err)
		assert.Equal(t, int64(0), matched)
		assert.Equal(t, int64(0), modified)

		foos = *tester.FindAll(&fooModel{}).(*[]*fooModel)
		assert.Equal(t, []*fooModel{
			{
				Base: foos[0].Base,
				Name: "foo",
				Foos: []fooModel{
					{},
					{},
				},
			},
			{
				Base: foos[1].Base,
				Name: "bar",
				Foos: []fooModel{
					{},
					{},
				},
			},
		}, foos)
	})
}
