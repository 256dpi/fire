package coal

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Tester provides facilities to work with coal models in tests.
type Tester struct {
	// The store to use for cleaning the database.
	Store *Store

	// The registered models.
	Models []Model
}

// NewTester returns a new tester.
func NewTester(store *Store, models ...Model) *Tester {
	return &Tester{
		Store:  store,
		Models: models,
	}
}

// Clean will remove the collections of models that have been registered and
// reset the header map.
func (t *Tester) Clean() {
	for _, model := range t.Models {
		// remove all is faster than dropping the collection
		_, err := t.Store.C(model).DeleteMany(nil, bson.M{})
		if err != nil {
			panic(err)
		}
	}
}

// Save will save the specified model.
func (t *Tester) Save(model Model) Model {
	// initialize model
	model = Init(model)

	// insert to collection
	_, err := t.Store.C(model).InsertOne(nil, model)
	if err != nil {
		panic(err)
	}

	return model
}

// FindAll will return all saved models.
func (t *Tester) FindAll(model Model) interface{} {
	// initialize model
	model = Init(model)

	// find all documents
	list := model.Meta().MakeSlice()
	cursor, err := t.Store.C(model).Find(nil, bson.M{}, options.Find().SetSort(Sort("_id")))
	if err != nil {
		panic(err)
	}

	// get all results
	err = cursor.All(nil, list)
	if err != nil {
		panic(err)
	}

	// close cursor
	err = cursor.Close(nil)
	if err != nil {
		panic(err)
	}

	// initialize list
	InitSlice(list)

	return list
}

// FindLast will return the last saved model.
func (t *Tester) FindLast(model Model) Model {
	// find last document
	err := t.Store.C(model).FindOne(nil, bson.M{}, options.FindOne().SetSort(Sort("-_id"))).Decode(model)
	if err != nil {
		panic(err)
	}

	// initialize model
	Init(model)

	return model
}

// Count will count all saved models.
func (t *Tester) Count(model Model) int {
	// count all documents
	n, err := t.Store.C(model).CountDocuments(nil, bson.M{})
	if err != nil {
		panic(err)
	}

	return int(n)
}

// Fetch will return the saved model.
func (t *Tester) Fetch(model Model, id ID) Model {
	// find specific document
	err := t.Store.C(model).FindOne(nil, bson.M{
		"_id": id,
	}).Decode(model)
	if err != nil {
		panic(err)
	}

	// initialize model
	Init(model)

	return model
}

// Update will update the specified model.
func (t *Tester) Update(model Model) Model {
	// initialize model
	model = Init(model)

	// insert to collection
	_, err := t.Store.C(model).ReplaceOne(nil, bson.M{
		"_id": model.ID(),
	}, model)
	if err != nil {
		panic(err)
	}

	return model
}

// Delete will delete the specified model.
func (t *Tester) Delete(model Model) {
	// initialize model
	model = Init(model)

	// insert to collection
	_, err := t.Store.C(model).DeleteOne(nil, bson.M{
		"_id": model.ID(),
	})
	if err != nil {
		panic(err)
	}
}
