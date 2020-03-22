package coal

import (
	"context"
	"errors"
	"fmt"

	"github.com/256dpi/lungo/bsonkit"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/256dpi/fire/cinder"
)

type empty struct {
	Base `bson:",inline"`
}

// Level describes the safety level under which an operation should be executed.
type Level int

const (
	// Unsafe will allow running operations without a transaction that by
	// default require a transaction.
	Unsafe Level = -1

	// Default is the default level.
	Default Level = 0
)

func max(levels []Level) Level {
	// use default if none specified
	if len(levels) == 0 {
		return Default
	}

	// get highest level
	level := levels[0]
	for i := 1; i < len(levels); i++ {
		if levels[i] > level {
			level = levels[i]
		}
	}

	return level
}

// ErrTransactionRequired is returned if an operation would be unsafe to perform
// without a transaction.
var ErrTransactionRequired = errors.New("operation requires a transaction")

var incrementLock = bson.M{
	"$inc": bson.M{
		"_lk": 1,
	},
}

var returnAfterUpdate = options.FindOneAndUpdate().SetReturnDocument(options.After)

// Manager manages operations on collection of documents. It will validate
// operations and ensure that they are safe under the MongoDB guarantees.
type Manager struct {
	coll  *Collection
	trans *Translator
}

// C is a short-hand to access the mangers collection.
func (m *Manager) C() *Collection {
	return m.coll
}

// T is a short-hand to access the managers translator.
func (m *Manager) T() *Translator {
	return m.trans
}

// Find will find the document with the specified id. It will return whether
// a document has been found. Lock can be set to true to force a write lock on
// the document and prevent a stale read during a transaction.
//
// A transaction is required for locking.
func (m *Manager) Find(ctx context.Context, model Model, id ID, lock bool) (bool, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.Find")
	span.Log("id", id.Hex())
	defer span.Finish()

	// check lock
	if lock && !getKey(ctx, hasTransaction) {
		return false, ErrTransactionRequired
	}

	// check model
	if model == nil {
		model = &empty{}
	}

	// prepare filter
	filter := bson.M{
		"_id": id,
	}

	// find document
	var err error
	if lock {
		err = m.coll.FindOneAndUpdate(ctx, filter, incrementLock, returnAfterUpdate).Decode(model)
	} else {
		err = m.coll.FindOne(ctx, filter).Decode(model)
	}
	if IsMissing(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}

// FindFirst will find the first document that matches the specified filter. It
// will return whether a document has been found. Lock can be set to true to
// force a write lock on the document and prevent a stale read during a
// transaction.
//
// A transaction is required for locking.
//
// Warning: If the operation depends on interleaving writes to not include or
// exclude documents from the filter it should be run during a transaction.
func (m *Manager) FindFirst(ctx context.Context, model Model, filter bson.M, sort []string, skip int64, lock bool) (bool, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.FindFirst")
	span.Log("filter", filter)
	span.Log("sort", sort)
	span.Log("skip", skip)
	defer span.Finish()

	// check lock
	if lock && !getKey(ctx, hasTransaction) {
		return false, ErrTransactionRequired
	}

	// check lock
	if lock && (skip > 0) {
		return false, fmt.Errorf("cannot lock with skip")
	}

	// check model
	if model == nil {
		model = &empty{}
	}

	// translate filter
	filterDoc, err := m.trans.Document(filter)
	if err != nil {
		return false, err
	}

	// translate sort
	var sortDoc bson.D
	if len(sort) > 0 {
		sortDoc, err = m.trans.Sort(sort)
		if err != nil {
			return false, err
		}
	}

	// find document
	if lock {
		// prepare options
		opts := options.FindOneAndUpdate()
		if sortDoc != nil {
			opts.SetSort(sortDoc)
		}

		// find and update
		err = m.coll.FindOneAndUpdate(ctx, filterDoc, incrementLock, returnAfterUpdate, opts).Decode(model)
	} else {
		// prepare options
		opts := options.FindOne()
		if sortDoc != nil {
			opts.SetSort(sortDoc)
		}
		if skip > 0 {
			opts.SetSkip(skip)
		}

		// find
		err = m.coll.FindOne(ctx, filterDoc, opts).Decode(model)
	}
	if IsMissing(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}

// FindAll will find all documents that match the specified filter. Lock can be
// set to true to force a write lock on the documents and prevent a stale read
// during a transaction.
//
// A transaction is required to ensure isolation.
//
// Unsafe: The result may miss documents or include them multiple times if
// interleaving operations move the documents in the used index.
func (m *Manager) FindAll(ctx context.Context, list interface{}, filter bson.M, sort []string, skip, limit int64, lock bool, level ...Level) error {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.FindAll")
	span.Log("filter", filter)
	span.Log("sort", sort)
	span.Log("skip", skip)
	span.Log("limit", limit)
	defer span.Finish()

	// require transaction if locked or not unsafe
	if (lock || max(level) > Unsafe) && !getKey(ctx, hasTransaction) {
		return ErrTransactionRequired
	}

	// check lock
	if lock && (skip > 0 || limit > 0) {
		return fmt.Errorf("cannot lock with skip and limit")
	}

	// translate filter
	filterDoc, err := m.trans.Document(filter)
	if err != nil {
		return err
	}

	// prepare options
	opts := options.Find()

	// set sort
	if len(sort) > 0 {
		sortDoc, err := m.trans.Sort(sort)
		if err != nil {
			return err
		}

		opts.SetSort(sortDoc)
	}

	// set skip
	if skip > 0 {
		opts.SetSkip(skip)
	}

	// set limit
	if limit > 0 {
		opts.SetLimit(limit)
	}

	// lock documents
	if lock {
		_, err = m.coll.UpdateMany(ctx, filterDoc, incrementLock)
		if err != nil {
			return err
		}
	}

	// find documents
	err = m.coll.FindAll(ctx, list, filterDoc, opts)
	if err != nil {
		return err
	}

	return nil
}

// FindEach will find all documents that match the specified filter. Lock can be
// set to true to force a write lock on the documents and prevent a stale read
// during a transaction.
//
// A transaction is always required to ensure isolation.
//
// Unsafe: The result may miss documents or include them multiple times if
// interleaving operations move the documents in the used index.
func (m *Manager) FindEach(ctx context.Context, filter bson.M, sort []string, skip, limit int64, lock bool, level ...Level) (*Iterator, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.FindEach")
	span.Log("filter", filter)
	span.Log("sort", sort)
	span.Log("skip", skip)
	span.Log("limit", limit)

	// finish span on error
	var iter *Iterator
	defer func() {
		if iter == nil {
			span.Finish()
		}
	}()

	// require transaction if locked or not unsafe
	if (lock || max(level) > Unsafe) && !getKey(ctx, hasTransaction) {
		return nil, ErrTransactionRequired
	}

	// check lock
	if lock && (skip > 0 || limit > 0) {
		return nil, fmt.Errorf("cannot lock with skip and limit")
	}

	// translate filter
	filterDoc, err := m.trans.Document(filter)
	if err != nil {
		return nil, err
	}

	// prepare options
	opts := options.Find()

	// set sort
	if len(sort) > 0 {
		sortDoc, err := m.trans.Sort(sort)
		if err != nil {
			return nil, err
		}

		opts.SetSort(sortDoc)
	}

	// set skip
	if skip > 0 {
		opts.SetSkip(skip)
	}

	// set limit
	if limit > 0 {
		opts.SetLimit(limit)
	}

	// lock documents
	if lock {
		_, err = m.coll.UpdateMany(ctx, filterDoc, incrementLock)
		if err != nil {
			return nil, err
		}
	}

	// find documents
	iter, err = m.coll.Find(ctx, filterDoc, opts)
	if err != nil {
		return nil, err
	}

	// attach span
	iter.spans = append(iter.spans, span)

	return iter, nil
}

// Count will count the documents that match the specified filter. Lock can be
// set to true to force a write lock on the documents and prevent a stale read
// during a transaction.
//
// A transaction is always required to ensure isolation.
//
// Unsafe: The count may miss documents or include them multiple times if
// interleaving operations move the documents in the used index.
func (m *Manager) Count(ctx context.Context, filter bson.M, skip, limit int64, lock bool, level ...Level) (int64, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.Count")
	span.Log("filter", filter)
	span.Log("skip", skip)
	span.Log("limit", limit)
	defer span.Finish()

	// require transaction if locked or not unsafe
	if (lock || max(level) > Unsafe) && !getKey(ctx, hasTransaction) {
		return 0, ErrTransactionRequired
	}

	// check lock
	if lock && (skip > 0 || limit > 0) {
		return 0, fmt.Errorf("cannot lock with skip and limit")
	}

	// translate filter
	filterDoc, err := m.trans.Document(filter)
	if err != nil {
		return 0, err
	}

	// prepare options
	opts := options.Count()

	// set skip
	if skip > 0 {
		opts.SetSkip(skip)
	}

	// set limit
	if limit > 0 {
		opts.SetLimit(limit)
	}

	// update if locked
	if lock {
		res, err := m.coll.UpdateMany(ctx, filterDoc, incrementLock)
		if err != nil {
			return 0, err
		}

		return res.ModifiedCount, nil
	}

	// count documents
	count, err := m.coll.CountDocuments(ctx, filterDoc, opts)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// Distinct will find all documents that match the specified filter and collect
// the specified field. Lock can be set to true to force a write lock on the
// documents and prevent a stale read during a transaction.
//
// A transaction is required to ensure isolation.
//
// Unsafe: The result may miss documents or include them multiple times if
// interleaving operations move the documents in the used index.
func (m *Manager) Distinct(ctx context.Context, field string, filter bson.M, lock bool, level ...Level) ([]interface{}, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.Distinct")
	defer span.Finish()

	// require transaction if locked or not unsafe
	if (lock || max(level) > Unsafe) && !getKey(ctx, hasTransaction) {
		return nil, ErrTransactionRequired
	}

	// translate field
	field, err := m.trans.Field(field)
	if err != nil {
		return nil, err
	}

	// translate filter
	filterDoc, err := m.trans.Document(filter)
	if err != nil {
		return nil, err
	}

	// lock documents
	if lock {
		_, err = m.coll.UpdateMany(ctx, filterDoc, incrementLock)
		if err != nil {
			return nil, err
		}
	}

	// distinct
	result, err := m.coll.Distinct(ctx, field, filterDoc)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// Insert will insert the provided documents. If a document has a zero id a new
// id will be generated and assigned. The documents are inserted in order until
// an error is encountered.
func (m *Manager) Insert(ctx context.Context, models ...Model) error {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.Insert")
	defer span.Finish()

	// ensure ids
	for _, model := range models {
		if model.ID().IsZero() {
			model.GetBase().DocID = New()
		}
	}

	// get documents
	docs := make([]interface{}, 0, len(models))
	for _, model := range models {
		docs = append(docs, model)
	}

	// insert documents
	_, err := m.coll.InsertMany(ctx, docs, options.InsertMany().SetOrdered(true))
	if err != nil {
		return err
	}

	// TODO: The returned result is omitted as the list of ids is generated by
	//  the driver. See https://jira.mongodb.org/browse/GODRIVER-1363.

	return nil
}

// InsertIfMissing will insert the provided document if no document matched the
// provided filter. If the document has a zero id a new id will be generated and
// assigned. It will return whether a document has been inserted. The underlying
// upsert operation will merge the filter with the model fields. Lock can be set
// to true to force a write lock on the existing document and prevent a stale
// read during a transaction.
//
// A transaction is required for locking.
//
// Warning: Even with transactions there is a risk for duplicate inserts when
// the filter is not covered by a unique index.
func (m *Manager) InsertIfMissing(ctx context.Context, filter bson.M, model Model, lock bool) (bool, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.InsertIfMissing")
	span.Log("filter", filter)
	defer span.Finish()

	// require transaction
	if lock && !getKey(ctx, hasTransaction) {
		return false, ErrTransactionRequired
	}

	// translate filter
	filterDoc, err := m.trans.Document(filter)
	if err != nil {
		return false, err
	}

	// ensure id
	if model.ID().IsZero() {
		model.GetBase().DocID = New()
	}

	// prepare options
	opts := options.Update().SetUpsert(true)

	// prepare update
	update := bson.M{
		"$setOnInsert": model,
	}

	// increment lock
	if lock {
		update["$inc"] = bson.M{
			"_lk": 1,
		}
	}

	// upsert document
	res, err := m.coll.UpdateOne(ctx, filterDoc, update, opts)
	if err != nil {
		return false, err
	}

	return res.UpsertedCount == 1, nil
}

// Replace will replace the existing document with the provided one. It will
// return whether a document has been found. Lock can be set to true to force a
// write lock on the document and prevent a stale read during a transaction in
// case the replace did not change the document.
//
// A transaction is required for locking.
func (m *Manager) Replace(ctx context.Context, model Model, lock bool) (bool, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.Replace")
	defer span.Finish()

	// check id
	if model.ID().IsZero() {
		return false, fmt.Errorf("model has a zero id")
	}

	// require transaction
	if lock && !getKey(ctx, hasTransaction) {
		return false, ErrTransactionRequired
	}

	// increment lock manually
	if lock {
		model.GetBase().Lock += 1000
	}

	// replace document
	res, err := m.coll.ReplaceOne(ctx, bson.M{
		"_id": model.ID(),
	}, model)
	if err != nil {
		return false, err
	}

	return res.MatchedCount == 1, nil
}

// ReplaceFirst will replace the first document that matches the specified filter.
// It will return whether a document has been found. Lock can be set to true to
// force a write lock on the document and prevent a stale read during a
// transaction if the replace did not cause an update.
//
// A transaction is required for locking.
//
// Warning: If the operation depends on interleaving writes to not include or
// exclude documents from the filter it should be run as part of a transaction.
func (m *Manager) ReplaceFirst(ctx context.Context, filter bson.M, model Model, lock bool) (bool, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.ReplaceFirst")
	span.Log("filter", filter)
	defer span.Finish()

	// require transaction
	if lock && !getKey(ctx, hasTransaction) {
		return false, ErrTransactionRequired
	}

	// increment lock manually
	if lock {
		model.GetBase().Lock += 1000
	}

	// translate filter
	filterDoc, err := m.trans.Document(filter)
	if err != nil {
		return false, err
	}

	// replace document
	res, err := m.coll.ReplaceOne(ctx, filterDoc, model)
	if err != nil {
		return false, err
	}

	return res.MatchedCount == 1, nil
}

// Update will update the document with the specified id. It will return whether
// a document has been found. Lock can be set to true to force a write lock on
// the document and prevent a stale read during a transaction in case the
// update did not chang the document.
//
// A transaction is required for locking.
func (m *Manager) Update(ctx context.Context, model Model, id ID, update bson.M, lock bool) (bool, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.Update")
	span.Log("id", id.Hex())
	span.Log("update", update)
	defer span.Finish()

	// require transaction
	if lock && !getKey(ctx, hasTransaction) {
		return false, ErrTransactionRequired
	}

	// translate update
	updateDoc, err := m.trans.Document(update)
	if err != nil {
		return false, err
	}

	// increment lock
	if lock {
		_, err := bsonkit.Put(&updateDoc, "$inc._lk", 1, false)
		if err != nil {
			return false, fmt.Errorf("unable to add lock: %w", err)
		}
	}

	// update document
	if model == nil {
		res, err := m.coll.UpdateOne(ctx, bson.M{
			"_id": id,
		}, updateDoc)
		if err != nil {
			return false, err
		}

		return res.MatchedCount == 1, nil
	}

	// find and update document
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	err = m.coll.FindOneAndUpdate(ctx, bson.M{
		"_id": id,
	}, updateDoc, opts).Decode(model)
	if IsMissing(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}

// UpdateFirst will update the first document that matches the specified filter.
// It will return whether a document has been found. Lock can be set to true to
// force a write lock on the document and prevent a stale read during a
// transaction in case the update did not change the document.
//
// A transaction is required for locking.
//
// Warning: If the operation depends on interleaving writes to not include or
// exclude documents from the filter it should be run as part of a transaction.
func (m *Manager) UpdateFirst(ctx context.Context, model Model, filter, update bson.M, sort []string, lock bool) (bool, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.UpdateFirst")
	span.Log("filter", filter)
	span.Log("update", update)
	defer span.Finish()

	// require transaction
	if lock && !getKey(ctx, hasTransaction) {
		return false, ErrTransactionRequired
	}

	// check model
	if model == nil {
		model = &empty{}
	}

	// translate filter
	filterDoc, err := m.trans.Document(filter)
	if err != nil {
		return false, err
	}

	// translate update
	updateDoc, err := m.trans.Document(update)
	if err != nil {
		return false, err
	}

	// prepare options
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	// set sort
	if len(sort) > 0 {
		sortDoc, err := m.trans.Sort(sort)
		if err != nil {
			return false, err
		}

		opts.SetSort(sortDoc)
	}

	// increment lock
	if lock {
		_, err := bsonkit.Put(&updateDoc, "$inc._lk", 1, false)
		if err != nil {
			return false, fmt.Errorf("unable to add lock: %w", err)
		}
	}

	// find and update document
	err = m.coll.FindOneAndUpdate(ctx, filterDoc, updateDoc, opts).Decode(model)
	if IsMissing(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}

// UpdateAll will update the documents that match the specified filter. It will
// return the number of matched documents. Lock can be set to true to force a
// write lock on the documents and prevent a stale read during a transaction in
// case the operation did not change all documents.
//
// A transaction is required for locking.
//
// Warning: If the operation depends on interleaving writes to not include or
// exclude documents from the filter it should be run as part of a transaction.
func (m *Manager) UpdateAll(ctx context.Context, filter, update bson.M, lock bool) (int64, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.UpdateAll")
	span.Log("filter", filter)
	span.Log("update", update)
	defer span.Finish()

	// require transaction
	if lock && !getKey(ctx, hasTransaction) {
		return 0, ErrTransactionRequired
	}

	// translate filter
	filterDoc, err := m.trans.Document(filter)
	if err != nil {
		return 0, err
	}

	// translate update
	updateDoc, err := m.trans.Document(update)
	if err != nil {
		return 0, err
	}

	// increment lock
	if lock {
		_, err := bsonkit.Put(&updateDoc, "$inc._lk", 1, false)
		if err != nil {
			return 0, fmt.Errorf("unable to add lock: %w", err)
		}
	}

	// update documents
	res, err := m.coll.UpdateMany(ctx, filterDoc, updateDoc)
	if err != nil {
		return 0, err
	}

	return res.MatchedCount, nil
}

// Upsert will update the first document that matches the specified filter. If
// no document has been found, the update document is applied to the filter and
// inserted. It will return whether a document has been inserted. Lock can be set
// to true to force a write lock on the existing document and prevent a stale
// read during a transaction.
//
// A transaction is required for locking.
//
// Warning: Even with transactions there is a risk for duplicate inserts when
// the filter is not covered by a unique index.
func (m *Manager) Upsert(ctx context.Context, model Model, filter, update bson.M, sort []string, lock bool) (bool, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.Upsert")
	span.Log("filter", filter)
	span.Log("update", update)
	defer span.Finish()

	// require transaction
	if lock && !getKey(ctx, hasTransaction) {
		return false, ErrTransactionRequired
	}

	// check model
	if model == nil {
		model = &empty{}
	}

	// translate filter
	filterDoc, err := m.trans.Document(filter)
	if err != nil {
		return false, err
	}

	// translate update
	updateDoc, err := m.trans.Document(update)
	if err != nil {
		return false, err
	}

	// increment lock
	if lock {
		_, err := bsonkit.Put(&updateDoc, "$inc._lk", 1, false)
		if err != nil {
			return false, fmt.Errorf("unable to add lock: %w", err)
		}
	}

	// prepare options
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)

	// set sort
	if len(sort) > 0 {
		sortDoc, err := m.trans.Sort(sort)
		if err != nil {
			return false, err
		}

		opts.SetSort(sortDoc)
	}

	// set token
	token := New()
	_, err = bsonkit.Put(&updateDoc, "$setOnInsert._tk", token, false)
	if err != nil {
		return false, fmt.Errorf("unable to set token: %w", err)
	}

	// find and update document
	err = m.coll.FindOneAndUpdate(ctx, filterDoc, updateDoc, opts).Decode(model)
	if IsMissing(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return model.GetBase().Token == token, nil
}

// Delete will remove the document with the specified id. It will return
// whether a document has been found and deleted.
func (m *Manager) Delete(ctx context.Context, model Model, id ID) (bool, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.Delete")
	span.Log("id", id.Hex())
	defer span.Finish()

	// delete document
	if model == nil {
		res, err := m.coll.DeleteOne(ctx, bson.M{
			"_id": id,
		})
		if err != nil {
			return false, err
		}

		return res.DeletedCount == 1, nil
	}

	// find and delete document
	err := m.coll.FindOneAndDelete(ctx, bson.M{
		"_id": id,
	}).Decode(model)
	if IsMissing(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}

// DeleteAll will delete the documents that match the specified filter. It will
// return the number of deleted documents.
//
// Warning: If the operation depends on interleaving writes to not include or
// exclude documents from the filter it should be run as part of a transaction.
func (m *Manager) DeleteAll(ctx context.Context, filter bson.M) (int64, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.DeleteAll")
	span.Log("filter", filter)
	defer span.Finish()

	// translate filter
	filterDoc, err := m.trans.Document(filter)
	if err != nil {
		return 0, err
	}

	// update documents
	res, err := m.coll.DeleteMany(ctx, filterDoc)
	if err != nil {
		return 0, err
	}

	return res.DeletedCount, nil
}

// DeleteFirst will delete the first document that matches the specified filter.
// It will return whether a document has been found and deleted.
//
// Warning: If the operation depends on interleaving writes to not include or
// exclude documents from the filter it should be run as part of a transaction.
func (m *Manager) DeleteFirst(ctx context.Context, model Model, filter bson.M, sort []string) (bool, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.DeleteFirst")
	span.Log("filter", filter)
	defer span.Finish()

	// translate filter
	filterDoc, err := m.trans.Document(filter)
	if err != nil {
		return false, err
	}

	// check model
	if model == nil {
		model = &empty{}
	}

	// prepare options
	opts := options.FindOneAndDelete()

	// set sort
	if len(sort) > 0 {
		sortDoc, err := m.trans.Sort(sort)
		if err != nil {
			return false, err
		}

		opts.SetSort(sortDoc)
	}

	// find and delete document
	err = m.coll.FindOneAndDelete(ctx, filterDoc, opts).Decode(model)
	if IsMissing(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}
