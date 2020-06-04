package glut

import (
	"context"
	"time"

	"github.com/256dpi/xo"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

// Lock will lock and read the specified value. Lock may create a new value in
// the process and lock it right away. It will also update the deadline of the
// value if a time to live is set.
func Lock(ctx context.Context, store *coal.Store, value Value, timeout time.Duration) (bool, error) {
	// trace
	ctx, span := xo.Trace(ctx, "glut/Lock")
	span.Tag("timeout", timeout.String())
	defer span.End()

	// get meta
	meta := GetMeta(value)

	// get base
	base := value.GetBase()

	// get key
	key, err := GetKey(value)
	if err != nil {
		return false, err
	}

	// ensure token
	if base.Token.IsZero() {
		base.Token = coal.New()
	}

	// log key, ttl and token
	span.Tag("key", key)
	span.Tag("ttl", meta.TTL.String())
	span.Tag("token", base.Token.Hex())

	// check timeout
	if timeout == 0 {
		return false, xo.F("missing timeout")
	}

	// check TTL
	if meta.TTL > 0 && meta.TTL < timeout {
		return false, xo.F("timeout greater than time to live")
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
		Key:      key,
		Data:     nil,
		Deadline: deadline,
		Locked:   &locked,
		Token:    coal.P(base.Token),
	}

	// insert value if missing
	inserted, err := store.M(&Model{}).InsertIfMissing(ctx, bson.M{
		"Key": key,
	}, &model, false)
	if err != nil {
		return false, err
	}

	// return if inserted
	if inserted {
		return true, nil
	}

	// update value
	found, err := store.M(&Model{}).UpdateFirst(ctx, &model, bson.M{
		"$and": []bson.M{
			{
				"Key": key,
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
						"Token": base.Token,
					},
				},
			},
		},
	}, bson.M{
		"$set": bson.M{
			"Deadline": deadline,
			"Locked":   locked,
			"Token":    base.Token,
		},
	}, nil, false)
	if err != nil {
		return false, err
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

	return found, nil
}

// SetLocked will update the specified value only if it is locked. It will als
// update the deadline of the value if a time to live is set.
func SetLocked(ctx context.Context, store *coal.Store, value Value) (bool, error) {
	// trace
	ctx, span := xo.Trace(ctx, "glut/SetLocked")
	defer span.End()

	// get meta
	meta := GetMeta(value)

	// get base
	base := value.GetBase()

	// get key
	key, err := GetKey(value)
	if err != nil {
		return false, err
	}

	// log key, ttl and token
	span.Tag("key", key)
	span.Tag("ttl", meta.TTL.String())
	span.Tag("token", base.Token.Hex())

	// check token
	if base.Token.IsZero() {
		return false, xo.F("missing token")
	}

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

	// encode data
	var data stick.Map
	err = data.Marshal(value, meta.Coding)
	if err != nil {
		return false, err
	}

	// update value
	found, err := store.M(&Model{}).UpdateFirst(ctx, nil, bson.M{
		"Key":   key,
		"Token": base.Token,
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

// GetLocked will load the contents of the specified value only if it is locked.
func GetLocked(ctx context.Context, store *coal.Store, value Value) (bool, error) {
	// trace
	ctx, span := xo.Trace(ctx, "glut/GetLocked")
	defer span.End()

	// get meta and base
	meta := GetMeta(value)
	base := value.GetBase()

	// get key
	key, err := GetKey(value)
	if err != nil {
		return false, err
	}

	// log key and token
	span.Tag("key", key)
	span.Tag("token", base.Token.Hex())

	// check token
	if base.Token.IsZero() {
		return false, xo.F("missing token")
	}

	// find value
	var model Model
	found, err := store.M(&Model{}).FindFirst(ctx, &model, bson.M{
		"Key":   key,
		"Token": base.Token,
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

// DelLocked will delete the specified value only if it is locked.
func DelLocked(ctx context.Context, store *coal.Store, value Value) (bool, error) {
	// trace
	ctx, span := xo.Trace(ctx, "glut/DelLocked")
	defer span.End()

	// get base
	base := value.GetBase()

	// get key
	key, err := GetKey(value)
	if err != nil {
		return false, err
	}

	// log key and token
	span.Tag("key", key)
	span.Tag("token", base.Token.Hex())

	// check token
	if base.Token.IsZero() {
		return false, xo.F("missing token")
	}

	// delete value
	deleted, err := store.M(&Model{}).DeleteFirst(ctx, nil, bson.M{
		"Key":   key,
		"Token": base.Token,
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
func MutLocked(ctx context.Context, store *coal.Store, value Value, fn func(bool) error) error {
	// trace
	ctx, span := xo.Trace(ctx, "glut/MutLocked")
	defer span.End()

	// get value
	ok, err := GetLocked(ctx, store, value)
	if err != nil {
		return err
	}

	// run function
	err = fn(ok)
	if err != nil {
		return err
	}

	// set value
	_, err = SetLocked(ctx, store, value)
	if err != nil {
		return err
	}

	return nil
}

// Unlock will unlock the specified value only if it is locked. It will also
// update the deadline of the value if a time to live is set.
func Unlock(ctx context.Context, store *coal.Store, value Value) (bool, error) {
	// trace
	ctx, span := xo.Trace(ctx, "glut/Unlock")
	defer span.End()

	// get meta
	meta := GetMeta(value)

	// get base
	base := value.GetBase()

	// get key
	key, err := GetKey(value)
	if err != nil {
		return false, err
	}

	// log key, ttl and token
	span.Tag("key", key)
	span.Tag("ttl", meta.TTL.String())
	span.Tag("token", base.Token.Hex())

	// check token
	if base.Token.IsZero() {
		return false, xo.F("missing token")
	}

	// prepare deadline
	var deadline *time.Time
	if meta.TTL > 0 {
		deadline = coal.T(time.Now().Add(meta.TTL))
	}

	// replace value
	found, err := store.M(&Model{}).UpdateFirst(ctx, nil, bson.M{
		"Key":   key,
		"Token": base.Token,
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
