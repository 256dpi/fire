package coal

import (
	"github.com/256dpi/xo"
	"go.mongodb.org/mongo-driver/bson"
)

// Tester provides facilities to work with coal models in tests.
type Tester struct {
	// The store to use for cleaning the database.
	Store *Store

	// The registered models.
	Models []Model
}

// NewTester returns a new tester. If no store is provided one will be created.
func NewTester(store *Store, models ...Model) *Tester {
	// ensure store
	if store == nil {
		store = MustOpen(nil, "test", xo.Panic)
	}

	// create tester
	tester := &Tester{
		Store:  store,
		Models: models,
	}

	// ensure collections
	for _, model := range models {
		_, _ = store.C(model).InsertOne(nil, GetMeta(model).Make())
	}

	// clean
	tester.Clean()

	return tester
}

// FindAll will return all saved models.
func (t *Tester) FindAll(model Model, query ...bson.M) interface{} {
	// prepare query
	qry := bson.M{}
	if len(query) > 0 {
		qry = query[0]
	}

	// find all documents
	list := GetMeta(model).MakeSlice()
	err := t.Store.M(model).FindAll(nil, list, qry, []string{"_id"}, 0, 0, false, NoTransaction)
	if err != nil {
		panic(err)
	}

	return list
}

// FindLast will return the last saved model.
func (t *Tester) FindLast(model Model, query ...bson.M) Model {
	// prepare query
	qry := bson.M{}
	if len(query) > 0 {
		qry = query[0]
	}

	// find last document
	found, err := t.Store.M(model).FindFirst(nil, model, qry, []string{"-_id"}, 0, false)
	if err != nil {
		panic(err)
	} else if !found {
		panic("not found")
	}

	return model
}

// Count will count all saved models.
func (t *Tester) Count(model Model, query ...bson.M) int {
	// prepare query
	qry := bson.M{}
	if len(query) > 0 {
		qry = query[0]
	}

	// count all documents
	n, err := t.Store.M(model).Count(nil, qry, 0, 0, false, NoTransaction)
	if err != nil {
		panic(err)
	}

	return int(n)
}

// Refresh will refresh the provided model.
func (t *Tester) Refresh(model Model) {
	// refresh model
	t.Fetch(model, model.ID())
}

// Fetch will return the saved model.
func (t *Tester) Fetch(model Model, id ID) Model {
	// find model
	found, err := t.Store.M(model).Find(nil, model, id, false)
	if err != nil {
		panic(err)
	} else if !found {
		panic("not found")
	}

	return model
}

// Insert will insert the specified model.
func (t *Tester) Insert(model Model) Model {
	// insert to collection
	err := t.Store.M(model).Insert(nil, model)
	if err != nil {
		panic(err)
	}

	return model
}

// Replace will replace the specified model.
func (t *Tester) Replace(model Model) Model {
	// replace model
	found, err := t.Store.M(model).Replace(nil, model, false)
	if err != nil {
		panic(err)
	} else if !found {
		panic("not found")
	}

	return model
}

// Update will update the specified model.
func (t *Tester) Update(model Model, update bson.M) Model {
	// replace model
	found, err := t.Store.M(model).Update(nil, model, model.ID(), update, false)
	if err != nil {
		panic(err)
	} else if !found {
		panic("not found")
	}

	return model
}

// Delete will delete the specified model.
func (t *Tester) Delete(model Model) {
	// delete model
	found, err := t.Store.M(model).Delete(nil, nil, model.ID())
	if err != nil {
		panic(err)
	} else if !found {
		panic("not found")
	}
}

// Clean will remove the collections of models that have been registered and
// reset the header map.
func (t *Tester) Clean() {
	for _, model := range t.Models {
		// remove all is faster than dropping the collection
		_, err := t.Store.M(model).DeleteAll(nil, bson.M{})
		if err != nil {
			panic(err)
		}
	}
}
