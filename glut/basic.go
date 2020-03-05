package glut

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/cinder"
	"github.com/256dpi/fire/coal"
)

// Get will load the contents of the value with the specified name. It will also
// return whether the value exists at all.
func Get(ctx context.Context, store *coal.Store, component, name string) (coal.Map, bool, error) {
	// track
	ctx, span := cinder.Track(ctx, "glut/Get")
	span.Tag("component", component)
	span.Log("name", name)
	defer span.Finish()

	// find value
	var value Value
	found, err := store.M(&Value{}).FindFirst(ctx, &value, bson.M{
		"Component": component,
		"Name":      name,
	}, nil, 0, false)
	if err != nil {
		return nil, false, err
	} else if !found {
		return nil, false, nil
	}

	return value.Data, true, nil
}

// Set will write the specified value to the store and overwrite any existing
// data. It will return if a new value has been created in the process. This
// method will ignore any locks held on the value.
func Set(ctx context.Context, store *coal.Store, component, name string, data coal.Map, ttl time.Duration) (bool, error) {
	// track
	ctx, span := cinder.Track(ctx, "glut/Set")
	span.Tag("component", component)
	span.Log("name", name)
	span.Log("ttl", ttl.String())
	defer span.Finish()

	// prepare deadline
	var deadline *time.Time
	if ttl > 0 {
		deadline = coal.T(time.Now().Add(ttl))
	}

	// upsert value
	inserted, err := store.M(&Value{}).Upsert(ctx, bson.M{
		"Component": component,
		"Name":      name,
	}, bson.M{
		"$set": bson.M{
			"Data":     data,
			"Deadline": deadline,
		},
	}, false)
	if err != nil {
		return false, err
	}

	return inserted, nil
}

// Del will remove the specified value from the store. This method will ignore
// any locks held on the value.
func Del(ctx context.Context, store *coal.Store, component, name string) (bool, error) {
	// track
	ctx, span := cinder.Track(ctx, "glut/Del")
	span.Tag("component", component)
	span.Log("name", name)
	defer span.Finish()

	// delete value
	deleted, err := store.M(&Value{}).DeleteFirst(ctx, nil, bson.M{
		"Component": component,
		"Name":      name,
	})
	if err != nil {
		return false, err
	}

	return deleted, nil
}

// Mut will load the specified value, run the callback and on success write the
// value back. This method will ignore any locks held on the value.
func Mut(ctx context.Context, store *coal.Store, component, name string, ttl time.Duration, fn func(bool, coal.Map) (coal.Map, error)) error {
	// track
	ctx, span := cinder.Track(ctx, "glut/Mut")
	span.Tag("component", component)
	span.Log("name", name)
	span.Log("ttl", ttl.String())
	defer span.Finish()

	// get value
	data, ok, err := Get(ctx, store, component, name)
	if err != nil {
		return err
	}

	// run function
	newData, err := fn(ok, data)
	if err != nil {
		return err
	}

	// put value
	_, err = Set(ctx, store, component, name, newData, ttl)
	if err != nil {
		return err
	}

	return nil
}
