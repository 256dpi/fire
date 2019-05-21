package coal

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// A Tester provides facilities to the test a fire API.
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
		_, err := t.Store.C(model).DeleteMany(context.Background(), bson.M{})
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
	_, err := t.Store.C(model).InsertOne(context.Background(), model)
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
	cursor, err := t.Store.C(model).Find(context.Background(), nil, options.Find().SetSort(Sort("_id")))
	if err != nil {
		panic(err)
	}

	// get all results
	err = cursor.All(context.Background(), list)
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
	err := t.Store.C(model).FindOne(context.Background(), nil, options.FindOne().SetSort(Sort("_id"))).Decode(model)
	if err != nil {
		panic(err)
	}

	// initialize model
	Init(model)

	return model
}

// Fetch will return the saved model.
func (t *Tester) Fetch(model Model, id primitive.ObjectID) Model {
	// find specific document
	err := t.Store.C(model).FindOne(context.Background(), bson.M{
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
	_, err := t.Store.C(model).ReplaceOne(context.Background(), bson.M{
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
	_, err := t.Store.C(model).DeleteOne(context.Background(), bson.M{
		"_id": model.ID(),
	})
	if err != nil {
		panic(err)
	}
}
