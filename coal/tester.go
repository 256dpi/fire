package coal

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
	store := t.Store.Copy()
	defer store.Close()

	for _, model := range t.Models {
		// remove all is faster than dropping the collection
		_, err := store.C(model).RemoveAll(nil)
		if err != nil {
			panic(err)
		}
	}
}

// Save will save the specified model.
func (t *Tester) Save(model Model) Model {
	store := t.Store.Copy()
	defer store.Close()

	// initialize model
	model = Init(model)

	// insert to collection
	err := store.C(model).Insert(model)
	if err != nil {
		panic(err)
	}

	return model
}

// FindAll will return all saved models.
func (t *Tester) FindAll(model Model) interface{} {
	store := t.Store.Copy()
	defer store.Close()

	// initialize model
	model = Init(model)

	// find all documents
	list := model.Meta().MakeSlice()
	err := store.C(model).Find(nil).Sort("-_id").All(list)
	if err != nil {
		panic(err)
	}

	// initialize list
	InitSlice(list)

	return list
}

// FindLast will return the last saved model.
func (t *Tester) FindLast(model Model) Model {
	store := t.Store.Copy()
	defer store.Close()

	// find last document
	err := store.C(model).Find(nil).Sort("-_id").One(model)
	if err != nil {
		panic(err)
	}

	// initialize model
	Init(model)

	return model
}

// Update will update the specified model.
func (t *Tester) Update(model Model) Model {
	store := t.Store.Copy()
	defer store.Close()

	// initialize model
	model = Init(model)

	// insert to collection
	err := store.C(model).UpdateId(model.ID(), model)
	if err != nil {
		panic(err)
	}

	return model
}

// Delete will delete the specified model.
func (t *Tester) Delete(model Model) {
	store := t.Store.Copy()
	defer store.Close()

	// initialize model
	model = Init(model)

	// insert to collection
	err := store.C(model).RemoveId(model.ID())
	if err != nil {
		panic(err)
	}
}
