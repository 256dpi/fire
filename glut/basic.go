package glut

import (
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/256dpi/fire/coal"
)

// Get will load the contents of the value with the specified name. It will also
// return whether the value exists at all.
func Get(store *coal.Store, component, name string) (coal.Map, bool, error) {
	// find value
	var value *Value
	err := store.C(&Value{}).FindOne(nil, bson.M{
		coal.F(&Value{}, "Component"): component,
		coal.F(&Value{}, "Name"):      name,
	}).Decode(&value)
	if err == mongo.ErrNoDocuments {
		return nil, false, nil
	} else if err != nil {
		return nil, false, err
	}

	return value.Data, true, nil
}

// Set will write the specified value to the store and overwrite any existing
// data. It will return if a new value has been created in the process. This
// method will ignore any locks held on the value.
func Set(store *coal.Store, component, name string, data coal.Map, ttl time.Duration) (bool, error) {
	// prepare deadline
	var deadline *time.Time
	if ttl > 0 {
		deadline = coal.T(time.Now().Add(ttl))
	}

	// update value
	res, err := store.C(&Value{}).UpdateOne(nil, bson.M{
		coal.F(&Value{}, "Component"): component,
		coal.F(&Value{}, "Name"):      name,
	}, bson.M{
		"$set": bson.M{
			coal.F(&Value{}, "Data"):     data,
			coal.F(&Value{}, "Deadline"): deadline,
		},
	}, options.Update().SetUpsert(true))
	if err != nil {
		return false, err
	}

	return res.UpsertedCount > 0, nil
}

// Del will remove the specified value from the store. This method will ignore
// any locks held on the value.
func Del(store *coal.Store, component, name string) (bool, error) {
	// delete value
	res, err := store.C(&Value{}).DeleteOne(nil, bson.M{
		coal.F(&Value{}, "Component"): component,
		coal.F(&Value{}, "Name"):      name,
	})
	if err != nil {
		return false, err
	}

	return res.DeletedCount > 0, nil
}

// Mut will load the specified value, run the callback and on success write the
// value back. This method will ignore any locks held on the value.
func Mut(store *coal.Store, component, name string, ttl time.Duration, fn func(bool, coal.Map) (coal.Map, error)) error {
	// get value
	data, ok, err := Get(store, component, name)
	if err != nil {
		return err
	}

	// run function
	newData, err := fn(ok, data)
	if err != nil {
		return err
	}

	// put value
	_, err = Set(store, component, name, newData, ttl)
	if err != nil {
		return err
	}

	return nil
}
