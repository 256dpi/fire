package glut

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/cinder"
	"github.com/256dpi/fire/coal"
)

// Lock will lock the specified value using the provided token for the
// specified duration. Lock may create a new value in the process and lock it
// right away. It will also update the deadline of the value if a TTL is set.
// An existing values contents are read and decoded to the specified value.
func Lock(ctx context.Context, store *coal.Store, value Value, token coal.ID, timeout time.Duration) (bool, error) {
	// get meta
	meta := Meta(value)

	// track
	ctx, span := cinder.Track(ctx, "glut/Lock")
	span.Log("key", meta.Key)
	span.Log("ttl", meta.TTL.String())
	span.Log("token", token.Hex())
	span.Log("timeout", timeout.String())
	defer span.Finish()

	// check token
	if token.IsZero() {
		return false, fmt.Errorf("invalid token")
	}

	// check timeout
	if timeout == 0 {
		return false, fmt.Errorf("invalid timeout")
	}

	// check TTL
	if meta.TTL > 0 && meta.TTL < timeout {
		return false, fmt.Errorf("timeout greather than TTL")
	}

	// prepare deadline
	var deadline *time.Time
	if meta.TTL > 0 {
		deadline = coal.T(time.Now().Add(meta.TTL))
	}

	// compute locked
	locked := time.Now().Add(timeout)

	// prepare value
	model := Model{
		Key:      meta.Key,
		Data:     nil,
		Deadline: deadline,
		Locked:   &locked,
		Token:    &token,
	}

	// insert value if missing
	inserted, err := store.M(&Model{}).InsertIfMissing(ctx, bson.M{
		"Key": meta.Key,
	}, &model, false)
	if err != nil {
		return false, err
	}

	// check if locked by upsert
	if inserted {
		return true, nil
	}

	// update value
	found, err := store.M(&Model{}).UpdateFirst(ctx, &model, bson.M{
		"$and": []bson.M{
			{
				"Key": meta.Key,
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
			"Deadline": deadline,
			"Locked":   locked,
			"Token":    token,
		},
	}, nil, false)
	if err != nil {
		return false, err
	}

	// decode value
	err = model.Data.Unmarshal(value, coal.TransferJSON)
	if err != nil {
		return false, err
	}

	return found, nil
}

// SetLocked will update the specified value only if it is locked with the
// provided token. It will also update the deadline of the value if a TTL is set.
func SetLocked(ctx context.Context, store *coal.Store, value Value, token coal.ID) (bool, error) {
	// get meta
	meta := Meta(value)

	// track
	ctx, span := cinder.Track(ctx, "glut/SetLocked")
	span.Log("key", meta.Key)
	span.Log("ttl", meta.TTL.String())
	span.Log("token", token.Hex())
	defer span.Finish()

	// check token
	if token.IsZero() {
		return false, fmt.Errorf("invalid token")
	}

	// prepare deadline
	var deadline *time.Time
	if meta.TTL > 0 {
		deadline = coal.T(time.Now().Add(meta.TTL))
	}

	// encode data
	var data coal.Map
	err := data.Marshal(value, coal.TransferJSON)
	if err != nil {
		return false, err
	}

	// update value
	found, err := store.M(&Model{}).UpdateFirst(ctx, nil, bson.M{
		"Key":   meta.Key,
		"Token": token,
		"Locked": bson.M{
			"$gt": time.Now(),
		},
	}, bson.M{
		"$set": bson.M{
			"Data":     data,
			"Deadline": deadline,
		},
	}, nil, false)
	if err != nil {
		return false, err
	}

	return found, nil
}

// GetLocked will load the contents of the specified value only if it is locked
// with the provided token.
func GetLocked(ctx context.Context, store *coal.Store, value Value, token coal.ID) (bool, error) {
	// get meta
	meta := Meta(value)

	// track
	ctx, span := cinder.Track(ctx, "glut/GetLocked")
	span.Log("key", meta.Key)
	span.Log("token", token.Hex())
	defer span.Finish()

	// find value
	var model Model
	found, err := store.M(&Model{}).FindFirst(ctx, &model, bson.M{
		"Key":   meta.Key,
		"Token": token,
		"Locked": bson.M{
			"$gt": time.Now(),
		},
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

// DelLocked will delete the specified value only if it is locked with the
// provided token.
func DelLocked(ctx context.Context, store *coal.Store, value Value, token coal.ID) (bool, error) {
	// get meta
	meta := Meta(value)

	// track
	ctx, span := cinder.Track(ctx, "glut/DelLocked")
	span.Log("key", meta.Key)
	span.Log("token", token.Hex())
	defer span.Finish()

	// check token
	if token.IsZero() {
		return false, fmt.Errorf("invalid token")
	}

	// delete value
	deleted, err := store.M(&Model{}).DeleteFirst(ctx, nil, bson.M{
		"Key":   meta.Key,
		"Token": token,
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
func MutLocked(ctx context.Context, store *coal.Store, value Value, token coal.ID, fn func(bool) error) error {
	// track
	ctx, span := cinder.Track(ctx, "glut/MutLocked")
	defer span.Finish()

	// get value
	ok, err := GetLocked(ctx, store, value, token)
	if err != nil {
		return err
	}

	// run function
	err = fn(ok)
	if err != nil {
		return err
	}

	// set value
	_, err = SetLocked(ctx, store, value, token)
	if err != nil {
		return err
	}

	return nil
}

// Unlock will unlock the specified value only if it is locked with the provided
// token. It will also update the deadline of the value if a TTL is set.
func Unlock(ctx context.Context, store *coal.Store, value Value, token coal.ID) (bool, error) {
	// get meta
	meta := Meta(value)

	// track
	ctx, span := cinder.Track(ctx, "glut/Unlock")
	span.Log("key", meta.Key)
	span.Log("ttl", meta.TTL.String())
	span.Log("token", token.Hex())
	defer span.Finish()

	// check token
	if token.IsZero() {
		return false, fmt.Errorf("invalid token")
	}

	// prepare deadline
	var deadline *time.Time
	if meta.TTL > 0 {
		deadline = coal.T(time.Now().Add(meta.TTL))
	}

	// replace value
	found, err := store.M(&Model{}).UpdateFirst(ctx, nil, bson.M{
		"Key":   meta.Key,
		"Token": token,
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
