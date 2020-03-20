package glut

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/cinder"
	"github.com/256dpi/fire/coal"
)

// Get will load the contents of the specified value. It will also return
// whether the value exists at all.
func Get(ctx context.Context, store *coal.Store, value Value) (bool, error) {
	// get meta
	meta := Meta(value)

	// prepare key
	key := meta.Key
	extension, err := value.GetExtension()
	if err != nil {
		return false, err
	} else if extension != "" {
		key += extension
	}

	// track
	ctx, span := cinder.Track(ctx, "glut/Get")
	span.Log("key", key)
	defer span.Finish()

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
	err = model.Data.Unmarshal(value, coal.TransferJSON)
	if err != nil {
		return false, err
	}

	return true, nil
}

// Set will write the specified value to the store and overwrite any existing
// data. It will return if a new value has been created in the process. This
// method will ignore any locks held on the value.
func Set(ctx context.Context, store *coal.Store, value Value) (bool, error) {
	// get meta
	meta := Meta(value)

	// prepare key
	key := meta.Key
	extension, err := value.GetExtension()
	if err != nil {
		return false, err
	} else if extension != "" {
		key += extension
	}

	// track
	ctx, span := cinder.Track(ctx, "glut/Set")
	span.Log("key", key)
	span.Log("ttl", meta.TTL.String())
	defer span.Finish()

	// prepare deadline
	var deadline *time.Time
	if meta.TTL > 0 {
		deadline = coal.T(time.Now().Add(meta.TTL))
	}

	// encode value
	var data coal.Map
	err = data.Marshal(value, coal.TransferJSON)
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
	// get meta
	meta := Meta(value)

	// prepare key
	key := meta.Key
	extension, err := value.GetExtension()
	if err != nil {
		return false, err
	} else if extension != "" {
		key += extension
	}

	// track
	ctx, span := cinder.Track(ctx, "glut/Del")
	span.Log("key", key)
	defer span.Finish()

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
	// track
	ctx, span := cinder.Track(ctx, "glut/Mut")
	defer span.Finish()

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
