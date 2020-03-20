package glut

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/cinder"
	"github.com/256dpi/fire/coal"
)

// Lock will lock the specified value using the specified token for the
// specified duration. Lock may create a new value in the process and lock it
// right away. It will also update the deadline of the value if TTL is set.
func Lock(ctx context.Context, store *coal.Store, component, name string, token coal.ID, timeout, ttl time.Duration) (bool, error) {
	// track
	ctx, span := cinder.Track(ctx, "glut/Lock")
	span.Tag("component", component)
	span.Log("name", name)
	span.Log("token", token.Hex())
	span.Log("timeout", timeout.String())
	span.Log("ttl", ttl.String())
	defer span.Finish()

	// check token
	if token.IsZero() {
		return false, fmt.Errorf("invalid token")
	}

	// check timeout
	if timeout == 0 {
		return false, fmt.Errorf("invalid timeout")
	}

	// check ttl
	if ttl > 0 && ttl < timeout {
		return false, fmt.Errorf("invalid ttl")
	}

	// prepare deadline
	var deadline *time.Time
	if ttl > 0 {
		deadline = coal.T(time.Now().Add(ttl))
	}

	// get locked
	locked := time.Now().Add(timeout)

	// upsert value
	inserted, err := store.M(&Model{}).Upsert(ctx, nil, bson.M{
		"Component": component,
		"Name":      name,
	}, bson.M{
		"$setOnInsert": bson.M{
			"Locked":   locked,
			"Token":    token,
			"Deadline": deadline,
		},
	}, nil, false)
	if err != nil {
		return false, err
	}

	// check if locked by upsert
	if inserted {
		return true, nil
	}

	// lock value
	found, err := store.M(&Model{}).UpdateFirst(ctx, nil, bson.M{
		"$and": []bson.M{
			{
				"Component": component,
				"Name":      name,
			},
			{
				"$or": []bson.M{
					// unlocked
					{
						"Token": nil,
					},
					// lock timed out
					{
						"Locked": bson.M{
							"$lt": time.Now(),
						},
					},
					// we have the lock
					{
						"Token": token,
					},
				},
			},
		},
	}, bson.M{
		"$set": bson.M{
			"Locked":   locked,
			"Token":    token,
			"Deadline": deadline,
		},
	}, nil, false)
	if err != nil {
		return false, err
	}

	return found, nil
}

// SetLocked will update the specified value only if the value is locked by the
// specified token.
func SetLocked(ctx context.Context, store *coal.Store, component, name string, data coal.Map, token coal.ID) (bool, error) {
	// track
	ctx, span := cinder.Track(ctx, "glut/SetLocked")
	span.Tag("component", component)
	span.Log("name", name)
	span.Log("token", token.Hex())
	defer span.Finish()

	// check token
	if token.IsZero() {
		return false, fmt.Errorf("invalid token")
	}

	// update value
	found, err := store.M(&Model{}).UpdateFirst(ctx, nil, bson.M{
		"Component": component,
		"Name":      name,
		"Token":     token,
		"Locked": bson.M{
			"$gt": time.Now(),
		},
	}, bson.M{
		"$set": bson.M{
			"Data": data,
		},
	}, nil, false)
	if err != nil {
		return false, err
	}

	return found, nil
}

// GetLocked will load the contents of the value with the specified name only
// if the value is locked by the specified token.
func GetLocked(ctx context.Context, store *coal.Store, component, name string, token coal.ID) (coal.Map, bool, error) {
	// track
	ctx, span := cinder.Track(ctx, "glut/GetLocked")
	span.Tag("component", component)
	span.Log("name", name)
	span.Log("token", token.Hex())
	defer span.Finish()

	// find value
	var value Model
	found, err := store.M(&Model{}).FindFirst(ctx, &value, bson.M{
		"Component": component,
		"Name":      name,
		"Token":     token,
		"Locked": bson.M{
			"$gt": time.Now(),
		},
	}, nil, 0, false)
	if err != nil {
		return nil, false, err
	} else if !found {
		return nil, false, nil
	}

	return value.Data, true, nil
}

// DelLocked will update the specified value only if the value is locked by the
// specified token.
func DelLocked(ctx context.Context, store *coal.Store, component, name string, token coal.ID) (bool, error) {
	// track
	ctx, span := cinder.Track(ctx, "glut/DelLocked")
	span.Tag("component", component)
	span.Log("name", name)
	span.Log("token", token.Hex())
	defer span.Finish()

	// check token
	if token.IsZero() {
		return false, fmt.Errorf("invalid token")
	}

	// delete value
	deleted, err := store.M(&Model{}).DeleteFirst(ctx, nil, bson.M{
		"Component": component,
		"Name":      name,
		"Token":     token,
		"Locked": bson.M{
			"$gt": time.Now(),
		},
	}, nil)
	if err != nil {
		return false, err
	}

	return deleted, nil
}

// MutLocked will load the specified value, run the callback and on success
// write the value back.
func MutLocked(ctx context.Context, store *coal.Store, component, name string, token coal.ID, fn func(bool, coal.Map) (coal.Map, error)) error {
	// track
	ctx, span := cinder.Track(ctx, "glut/MutLocked")
	span.Tag("component", component)
	span.Log("name", name)
	span.Log("token", token.Hex())
	defer span.Finish()

	// get value
	data, ok, err := GetLocked(ctx, store, component, name, token)
	if err != nil {
		return err
	}

	// run function
	newData, err := fn(ok, data)
	if err != nil {
		return err
	}

	// put value
	_, err = SetLocked(ctx, store, component, name, newData, token)
	if err != nil {
		return err
	}

	return nil
}

// Unlock will unlock the specified value if the provided token does match. It
// will also update the deadline of the value if TTL is set.
func Unlock(ctx context.Context, store *coal.Store, component, name string, token coal.ID, ttl time.Duration) (bool, error) {
	// check token
	if token.IsZero() {
		return false, fmt.Errorf("invalid token")
	}

	// prepare deadline
	var deadline *time.Time
	if ttl > 0 {
		deadline = coal.T(time.Now().Add(ttl))
	}

	// replace value
	found, err := store.M(&Model{}).UpdateFirst(ctx, nil, bson.M{
		"Component": component,
		"Name":      name,
		"Token":     token,
		"Locked": bson.M{
			"$gt": time.Now(),
		},
	}, bson.M{
		"$set": bson.M{
			"Locked":   nil,
			"Token":    nil,
			"Deadline": deadline,
		},
	}, nil, false)
	if err != nil {
		return false, err
	}

	return found, nil
}
