package glut

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

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

	// ensure value
	res, err := store.C(&Value{}).UpdateOne(ctx, bson.M{
		coal.F(&Value{}, "Component"): component,
		coal.F(&Value{}, "Name"):      name,
	}, bson.M{
		"$setOnInsert": bson.M{
			coal.F(&Value{}, "Locked"):   locked,
			coal.F(&Value{}, "Token"):    token,
			coal.F(&Value{}, "Deadline"): deadline,
		},
	}, options.Update().SetUpsert(true))
	if err != nil {
		return false, err
	}

	// check if locked by upsert
	if res.UpsertedCount > 0 {
		return true, nil
	}

	// lock value
	res, err = store.C(&Value{}).UpdateOne(ctx, bson.M{
		"$and": []bson.M{
			{
				coal.F(&Value{}, "Component"): component,
				coal.F(&Value{}, "Name"):      name,
			},
			{
				"$or": []bson.M{
					// unlocked
					{
						coal.F(&Value{}, "Token"): nil,
					},
					// lock timed out
					{
						coal.F(&Value{}, "Locked"): bson.M{
							"$lt": time.Now(),
						},
					},
					// we have the lock
					{
						coal.F(&Value{}, "Token"): token,
					},
				},
			},
		},
	}, bson.M{
		"$set": bson.M{
			coal.F(&Value{}, "Locked"):   locked,
			coal.F(&Value{}, "Token"):    token,
			coal.F(&Value{}, "Deadline"): deadline,
		},
	})
	if err != nil {
		return false, err
	}

	return res.ModifiedCount > 0, nil
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
	res, err := store.C(&Value{}).UpdateOne(ctx, bson.M{
		coal.F(&Value{}, "Component"): component,
		coal.F(&Value{}, "Name"):      name,
		coal.F(&Value{}, "Token"):     token,
		coal.F(&Value{}, "Locked"): bson.M{
			"$gt": time.Now(),
		},
	}, bson.M{
		"$set": bson.M{
			coal.F(&Value{}, "Data"): data,
		},
	})
	if err != nil {
		return false, err
	}

	return res.ModifiedCount > 0, nil
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
	var value *Value
	err := store.C(&Value{}).FindOne(ctx, bson.M{
		coal.F(&Value{}, "Component"): component,
		coal.F(&Value{}, "Name"):      name,
		coal.F(&Value{}, "Token"):     token,
		coal.F(&Value{}, "Locked"): bson.M{
			"$gt": time.Now(),
		},
	}).Decode(&value)
	if coal.IsMissing(err) {
		return nil, false, nil
	} else if err != nil {
		return nil, false, err
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
	res, err := store.C(&Value{}).DeleteOne(ctx, bson.M{
		coal.F(&Value{}, "Component"): component,
		coal.F(&Value{}, "Name"):      name,
		coal.F(&Value{}, "Token"):     token,
		coal.F(&Value{}, "Locked"): bson.M{
			"$gt": time.Now(),
		},
	})
	if err != nil {
		return false, err
	}

	return res.DeletedCount > 0, nil
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
	res, err := store.C(&Value{}).UpdateOne(ctx, bson.M{
		coal.F(&Value{}, "Component"): component,
		coal.F(&Value{}, "Name"):      name,
		coal.F(&Value{}, "Token"):     token,
		coal.F(&Value{}, "Locked"): bson.M{
			"$gt": time.Now(),
		},
	}, bson.M{
		"$set": bson.M{
			coal.F(&Value{}, "Locked"):   nil,
			coal.F(&Value{}, "Token"):    nil,
			coal.F(&Value{}, "Deadline"): deadline,
		},
	})
	if err != nil {
		return false, err
	}

	return res.ModifiedCount > 0, nil
}
