package coal

import (
	"context"
	"errors"
	"reflect"

	"github.com/256dpi/lungo"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/256dpi/fire/cinder"
)

// IsMissing returns whether the provided error describes a missing document.
func IsMissing(err error) bool {
	return err == lungo.ErrNoDocuments || errors.Is(err, lungo.ErrNoDocuments)
}

// IsDuplicate returns whether the provided error describes a duplicate document.
func IsDuplicate(err error) bool {
	return lungo.IsUniquenessError(err)
}

// Collection mimics a collection and adds tracing.
type Collection struct {
	coll lungo.ICollection
}

// Native will return the underlying native collection.
func (c *Collection) Native() lungo.ICollection {
	return c.coll
}

// Aggregate wraps the native Aggregate collection method and yields the
// returned cursor.
func (c *Collection) Aggregate(ctx context.Context, pipeline interface{}, opts ...*options.AggregateOptions) (*Iterator, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Collection.Aggregate")
	span.Log("pipeline", pipeline)

	// aggregate
	csr, err := c.coll.Aggregate(ctx, pipeline, opts...)
	if err != nil {
		span.Finish()
		return nil, err
	}

	// create iterator
	iterator := newIterator(ctx, csr, span)

	return iterator, nil
}

// AggregateAll wraps the native Aggregate collection method and decodes all
// documents to the provided slice.
func (c *Collection) AggregateAll(ctx context.Context, slicePtr interface{}, pipeline interface{}, opts ...*options.AggregateOptions) error {
	// track
	ctx, span := cinder.Track(ctx, "coal/Collection.AggregateAll")
	span.Log("pipeline", pipeline)
	defer span.Finish()

	// aggregate
	csr, err := c.coll.Aggregate(ctx, pipeline, opts...)
	if err != nil {
		return err
	}

	// decode all documents
	err = csr.All(ctx, slicePtr)
	if err != nil {
		_ = csr.Close(ctx)
		return err
	}

	// log result
	span.Log("loaded", reflect.ValueOf(slicePtr).Elem().Len())

	return nil
}

// BulkWrite wraps the native BulkWrite collection method.
func (c *Collection) BulkWrite(ctx context.Context, models []mongo.WriteModel, opts ...*options.BulkWriteOptions) (*mongo.BulkWriteResult, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Collection.BulkWrite")
	defer span.Finish()

	// bulk write
	res, err := c.coll.BulkWrite(ctx, models, opts...)
	if err != nil {
		return nil, err
	}

	// log result
	span.Log("inserted", res.InsertedCount)
	span.Log("matched", res.MatchedCount)
	span.Log("modified", res.ModifiedCount)
	span.Log("deleted", res.DeletedCount)
	span.Log("upserted", res.UpsertedCount)

	return res, nil
}

// CountDocuments wraps the native CountDocuments collection method.
func (c *Collection) CountDocuments(ctx context.Context, filter interface{}, opts ...*options.CountOptions) (int64, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Collection.CountDocuments")
	span.Log("filter", filter)
	defer span.Finish()

	// count documents
	count, err := c.coll.CountDocuments(ctx, filter, opts...)
	if err != nil {
		return 0, err
	}

	// log result
	span.Log("count", count)

	return count, nil
}

// DeleteMany wraps the native DeleteMany collection method.
func (c *Collection) DeleteMany(ctx context.Context, filter interface{}, opts ...*options.DeleteOptions) (*mongo.DeleteResult, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Collection.DeleteMany")
	span.Log("filter", filter)
	defer span.Finish()

	// delete many
	res, err := c.coll.DeleteMany(ctx, filter, opts...)
	if err != nil {
		return nil, err
	}

	// log result
	span.Log("deleted", res.DeletedCount)

	return res, nil
}

// DeleteOne wraps the native DeleteOne collection method.
func (c *Collection) DeleteOne(ctx context.Context, filter interface{}, opts ...*options.DeleteOptions) (*mongo.DeleteResult, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Collection.DeleteOne")
	span.Log("filter", filter)
	defer span.Finish()

	// delete one
	res, err := c.coll.DeleteOne(ctx, filter, opts...)
	if err != nil {
		return nil, err
	}

	// log result
	span.Log("deleted", res.DeletedCount == 1)

	return res, nil
}

// Distinct wraps the native Distinct collection method.
func (c *Collection) Distinct(ctx context.Context, field string, filter interface{}, opts ...*options.DistinctOptions) ([]interface{}, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Collection.Distinct")
	span.Log("field", field)
	span.Log("filter", filter)
	defer span.Finish()

	// distinct
	list, err := c.coll.Distinct(ctx, field, filter, opts...)
	if err != nil {
		return nil, err
	}

	// log result
	span.Log("length", len(list))

	return list, nil
}

// EstimatedDocumentCount wraps the native EstimatedDocumentCount collection method.
func (c *Collection) EstimatedDocumentCount(ctx context.Context, opts ...*options.EstimatedDocumentCountOptions) (int64, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Collection.EstimatedDocumentCount")
	defer span.Finish()

	// estimate count
	count, err := c.coll.EstimatedDocumentCount(ctx, opts...)
	if err != nil {
		return 0, err
	}

	// log result
	span.Log("count", count)

	return count, nil
}

// Find wraps the native Find collection method and yields the returned cursor.
func (c *Collection) Find(ctx context.Context, filter interface{}, opts ...*options.FindOptions) (*Iterator, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Collection.Find")
	span.Log("filter", filter)

	// find
	csr, err := c.coll.Find(ctx, filter, opts...)
	if err != nil {
		span.Finish()
		return nil, err
	}

	// create iterator
	iterator := newIterator(ctx, csr, span)

	return iterator, nil
}

// FindAll wraps the native Find collection method and decodes all documents to
// the provided slice.
func (c *Collection) FindAll(ctx context.Context, slicePtr interface{}, filter interface{}, opts ...*options.FindOptions) error {
	// track
	ctx, span := cinder.Track(ctx, "coal/Collection.FindAll")
	span.Log("filter", filter)
	defer span.Finish()

	// find
	csr, err := c.coll.Find(ctx, filter, opts...)
	if err != nil {
		return err
	}

	// decode all documents
	err = csr.All(ctx, slicePtr)
	if err != nil {
		_ = csr.Close(ctx)
		return err
	}

	// log result
	span.Log("loaded", reflect.ValueOf(slicePtr).Elem().Len())

	return nil
}

// FindOne wraps the native FindOne collection method.
func (c *Collection) FindOne(ctx context.Context, filter interface{}, opts ...*options.FindOneOptions) lungo.ISingleResult {
	// track
	ctx, span := cinder.Track(ctx, "coal/Collection.FindOne")
	span.Log("filter", filter)
	defer span.Finish()

	// find one
	res := c.coll.FindOne(ctx, filter, opts...)

	return res
}

// FindOneAndDelete wraps the native FindOneAndDelete collection method.
func (c *Collection) FindOneAndDelete(ctx context.Context, filter interface{}, opts ...*options.FindOneAndDeleteOptions) lungo.ISingleResult {
	// track
	ctx, span := cinder.Track(ctx, "coal/Collection.FindOneAndDelete")
	span.Log("filter", filter)
	defer span.Finish()

	// find one and delete
	res := c.coll.FindOneAndDelete(ctx, filter, opts...)

	return res
}

// FindOneAndReplace wraps the native FindOneAndReplace collection method.
func (c *Collection) FindOneAndReplace(ctx context.Context, filter interface{}, replacement interface{}, opts ...*options.FindOneAndReplaceOptions) lungo.ISingleResult {
	// track
	ctx, span := cinder.Track(ctx, "coal/Collection.FindOneAndReplace")
	span.Log("filter", filter)
	defer span.Finish()

	// find and replace one
	res := c.coll.FindOneAndReplace(ctx, filter, replacement, opts...)

	return res
}

// FindOneAndUpdate wraps the native FindOneAndUpdate collection method.
func (c *Collection) FindOneAndUpdate(ctx context.Context, filter interface{}, update interface{}, opts ...*options.FindOneAndUpdateOptions) lungo.ISingleResult {
	// track
	ctx, span := cinder.Track(ctx, "coal/Collection.FindOneAndUpdate")
	span.Log("filter", filter)
	defer span.Finish()

	// find one and update
	res := c.coll.FindOneAndUpdate(ctx, filter, update, opts...)

	return res
}

// InsertMany wraps the native InsertMany collection method.
func (c *Collection) InsertMany(ctx context.Context, documents []interface{}, opts ...*options.InsertManyOptions) (*mongo.InsertManyResult, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Collection.InsertMany")
	span.Log("count", len(documents))
	defer span.Finish()

	// insert many
	res, err := c.coll.InsertMany(ctx, documents, opts...)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// InsertOne wraps the native InsertOne collection method.
func (c *Collection) InsertOne(ctx context.Context, document interface{}, opts ...*options.InsertOneOptions) (*mongo.InsertOneResult, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Collection.InsertOne")
	defer span.Finish()

	// insert one
	res, err := c.coll.InsertOne(ctx, document, opts...)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// ReplaceOne wraps the native ReplaceOne collection method.
func (c *Collection) ReplaceOne(ctx context.Context, filter interface{}, replacement interface{}, opts ...*options.ReplaceOptions) (*mongo.UpdateResult, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Collection.ReplaceOne")
	span.Log("filter", filter)
	defer span.Finish()

	// replace one
	res, err := c.coll.ReplaceOne(ctx, filter, replacement, opts...)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// UpdateMany wraps the native UpdateMany collection method.
func (c *Collection) UpdateMany(ctx context.Context, filter interface{}, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Collection.UpdateMany")
	span.Log("filter", filter)
	defer span.Finish()

	// update many
	res, err := c.coll.UpdateMany(ctx, filter, update, opts...)
	if err != nil {
		return nil, err
	}

	// log result
	span.Log("matched", res.MatchedCount)
	span.Log("modified", res.ModifiedCount)
	span.Log("upserted", res.UpsertedCount)

	return res, nil
}

// UpdateOne wraps the native UpdateOne collection method.
func (c *Collection) UpdateOne(ctx context.Context, filter interface{}, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Collection.UpdateOne")
	span.Log("filter", filter)
	defer span.Finish()

	// update one
	res, err := c.coll.UpdateOne(ctx, filter, update, opts...)
	if err != nil {
		return nil, err
	}

	// log result
	span.Log("matched", res.MatchedCount == 1)
	span.Log("modified", res.ModifiedCount == 1)
	span.Log("upserted", res.UpsertedCount == 1)

	return res, nil
}

// Iterator manages the iteration over a cursor.
type Iterator struct {
	ctx     context.Context
	cursor  lungo.ICursor
	span    *cinder.Span
	counter int64
	error   error
}

func newIterator(ctx context.Context, cursor lungo.ICursor, span *cinder.Span) *Iterator {
	return &Iterator{
		ctx:    ctx,
		cursor: cursor,
		span:   span,
	}
}

// Next will load the next document from the cursor and if available return true.
// If it returns false the iteration must be stopped due to the cursor being
// exhausted or an error.
func (i *Iterator) Next() bool {
	// check error
	if i.error != nil {
		return false
	}

	// await next
	if !i.cursor.Next(i.ctx) {
		i.error = i.cursor.Err()
		i.Close()
		return false
	}

	// increment
	i.counter++

	return true
}

// Decode will decode the loaded document to the specified value.
func (i *Iterator) Decode(v interface{}) error {
	return i.cursor.Decode(v)
}

// Error return the first error encountered during iteration. It should always
// be checked after an iteration has finished to ensure there where no errors.
func (i *Iterator) Error() error {
	return i.error
}

// Close will close the underlying cursor. A call to it should be deferred right
// after obtaining an iterator. Close should be called also if the iterator is
// still valid but no longer used by the application.
func (i *Iterator) Close() {
	// close cursor if available
	if i.cursor != nil {
		_ = i.cursor.Close(i.ctx)
	}

	// finish span
	i.span.Log("loaded", i.counter)
	i.span.Finish()
}
