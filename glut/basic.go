package glut

import (
	"context"
	"time"

	"github.com/256dpi/xo"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

// Get will load the contents of the specified value. It will also return
// whether the value exists at all.
func Get(ctx context.Context, store *coal.Store, value Value) (bool, error) {
	// get meta
	meta := GetMeta(value)

	// trace
	ctx, span := xo.Trace(ctx, "glut/Get")
	defer span.End()

	// get key
	key, err := GetKey(value)
	if err != nil {
		return false, err
	}

	// log key
	span.Tag("key", key)

	// find value
	var model Model
	found, err := store.M(&Model{}).FindFirst(ctx, &model, bson.M{
		"Key": key,
	}, nil, 0, false)
	if err != nil {
		return false, err
	} else if !found {
		return false, nil
	}

	// decode value
	err = model.Data.Unmarshal(value, meta.Coding)
	if err != nil {
		return false, err
	}

	// validate value
	err = value.Validate()
	if err != nil {
		return false, err
	}

	return true, nil
}

// Set will write the specified value to the store and overwrite any existing
// data. It will return if a new value has been created in the process. This
// method will ignore any locks held on the value.
func Set(ctx context.Context, store *coal.Store, value Value) (bool, error) {
	// trace
	ctx, span := xo.Trace(ctx, "glut/Set")
	defer span.End()

	// get meta
	meta := GetMeta(value)

	// get key
	key, err := GetKey(value)
	if err != nil {
		return false, err
	}

	// log key and ttl
	span.Tag("key", key)
	span.Tag("ttl", meta.TTL.String())

	// prepare deadline
	var deadline *time.Time
	if meta.TTL > 0 {
		deadline = coal.T(time.Now().Add(meta.TTL))
	}

	// validate value
	err = value.Validate()
	if err != nil {
		return false, err
	}

	// encode value
	var data stick.Map
	err = data.Marshal(value, meta.Coding)
	if err != nil {
		return false, err
	}

	// upsert value
	inserted, err := store.M(&Model{}).Upsert(ctx, nil, bson.M{
		"Key": key,
	}, bson.M{
		"$set": bson.M{
			"Data":     data,
			"Deadline": deadline,
		},
	}, nil, false)
	if err != nil {
		return false, err
	}

	return inserted, nil
}

// Del will remove the specified value from the store. This method will ignore
// any locks held on the value.
func Del(ctx context.Context, store *coal.Store, value Value) (bool, error) {
	// trace
	ctx, span := xo.Trace(ctx, "glut/Del")
	defer span.End()

	// get key
	key, err := GetKey(value)
	if err != nil {
		return false, err
	}

	// log key
	span.Tag("key", key)

	// delete value
	deleted, err := store.M(&Model{}).DeleteFirst(ctx, nil, bson.M{
		"Key": key,
	}, nil)
	if err != nil {
		return false, err
	}

	return deleted, nil
}

// Mut will load the specified value, run the callback and on success write the
// value back. This method will ignore any locks held on the value.
func Mut(ctx context.Context, store *coal.Store, value Value, fn func(bool) error) error {
	// trace
	ctx, span := xo.Trace(ctx, "glut/Mut")
	defer span.End()

	// get value
	exists, err := Get(ctx, store, value)
	if err != nil {
		return err
	}

	// run function
	err = fn(exists)
	if err != nil {
		return err
	}

	// set value
	_, err = Set(ctx, store, value)
	if err != nil {
		return err
	}

	return nil
}
