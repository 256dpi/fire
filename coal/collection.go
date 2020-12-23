package coal

import (
	"context"
	"errors"
	"reflect"

	"github.com/256dpi/lungo"
	"github.com/256dpi/xo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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
	// trace
	ctx, span := xo.Trace(ctx, "coal/Collection.Aggregate")
	span.Tag("collection", c.coll.Name())

	// aggregate
	csr, err := c.coll.Aggregate(ctx, pipeline, opts...)
	if err != nil {
		span.End()
		return nil, xo.W(err)
	}

	// create iterator
	iterator := newIterator(ctx, csr, span)

	return iterator, nil
}

// BulkWrite wraps the native BulkWrite collection method.
func (c *Collection) BulkWrite(ctx context.Context, models []mongo.WriteModel, opts ...*options.BulkWriteOptions) (*mongo.BulkWriteResult, error) {
	// trace
	ctx, span := xo.Trace(ctx, "coal/Collection.BulkWrite")
	span.Tag("collection", c.coll.Name())
	defer span.End()

	// bulk write
	res, err := c.coll.BulkWrite(ctx, models, opts...)
	if err != nil {
		return nil, xo.W(err)
	}

	// log result
	span.Tag("inserted", res.InsertedCount)
	span.Tag("matched", res.MatchedCount)
	span.Tag("modified", res.ModifiedCount)
	span.Tag("deleted", res.DeletedCount)
	span.Tag("upserted", res.UpsertedCount)

	return res, nil
}

// CountDocuments wraps the native CountDocuments collection method.
func (c *Collection) CountDocuments(ctx context.Context, filter interface{}, opts ...*options.CountOptions) (int64, error) {
	// trace
	ctx, span := xo.Trace(ctx, "coal/Collection.CountDocuments")
	span.Tag("collection", c.coll.Name())
	defer span.End()

	// count documents
	count, err := c.coll.CountDocuments(ctx, filter, opts...)
	if err != nil {
		return 0, xo.W(err)
	}

	// log result
	span.Tag("count", count)

	return count, nil
}

// DeleteMany wraps the native DeleteMany collection method.
func (c *Collection) DeleteMany(ctx context.Context, filter interface{}, opts ...*options.DeleteOptions) (*mongo.DeleteResult, error) {
	// trace
	ctx, span := xo.Trace(ctx, "coal/Collection.DeleteMany")
	span.Tag("collection", c.coll.Name())
	defer span.End()

	// delete many
	res, err := c.coll.DeleteMany(ctx, filter, opts...)
	if err != nil {
		return nil, xo.W(err)
	}

	// log result
	span.Tag("deleted", res.DeletedCount)

	return res, nil
}

// DeleteOne wraps the native DeleteOne collection method.
func (c *Collection) DeleteOne(ctx context.Context, filter interface{}, opts ...*options.DeleteOptions) (*mongo.DeleteResult, error) {
	// trace
	ctx, span := xo.Trace(ctx, "coal/Collection.DeleteOne")
	span.Tag("collection", c.coll.Name())
	defer span.End()

	// delete one
	res, err := c.coll.DeleteOne(ctx, filter, opts...)
	if err != nil {
		return nil, xo.W(err)
	}

	// log result
	span.Tag("deleted", res.DeletedCount == 1)

	return res, nil
}

// Distinct wraps the native Distinct collection method.
func (c *Collection) Distinct(ctx context.Context, field string, filter interface{}, opts ...*options.DistinctOptions) ([]interface{}, error) {
	// trace
	ctx, span := xo.Trace(ctx, "coal/Collection.Distinct")
	span.Tag("collection", c.coll.Name())
	span.Tag("field", field)
	defer span.End()

	// distinct
	list, err := c.coll.Distinct(ctx, field, filter, opts...)
	if err != nil {
		return nil, xo.W(err)
	}

	// log result
	span.Tag("length", len(list))

	return list, nil
}

// EstimatedDocumentCount wraps the native EstimatedDocumentCount collection method.
func (c *Collection) EstimatedDocumentCount(ctx context.Context, opts ...*options.EstimatedDocumentCountOptions) (int64, error) {
	// trace
	ctx, span := xo.Trace(ctx, "coal/Collection.EstimatedDocumentCount")
	span.Tag("collection", c.coll.Name())
	defer span.End()

	// estimate count
	count, err := c.coll.EstimatedDocumentCount(ctx, opts...)
	if err != nil {
		return 0, xo.W(err)
	}

	// log result
	span.Tag("count", count)

	return count, nil
}

// Find wraps the native Find collection method and yields the returned cursor.
func (c *Collection) Find(ctx context.Context, filter interface{}, opts ...*options.FindOptions) (*Iterator, error) {
	// trace
	ctx, span := xo.Trace(ctx, "coal/Collection.Find")
	span.Tag("collection", c.coll.Name())

	// find
	csr, err := c.coll.Find(ctx, filter, opts...)
	if err != nil {
		span.End()
		return nil, xo.W(err)
	}

	// create iterator
	iterator := newIterator(ctx, csr, span)

	return iterator, nil
}

// FindOne wraps the native FindOne collection method.
func (c *Collection) FindOne(ctx context.Context, filter interface{}, opts ...*options.FindOneOptions) lungo.ISingleResult {
	// trace
	ctx, span := xo.Trace(ctx, "coal/Collection.FindOne")
	span.Tag("collection", c.coll.Name())
	defer span.End()

	// find one
	res := c.coll.FindOne(ctx, filter, opts...)

	return &SingleResult{res: res}
}

// FindOneAndDelete wraps the native FindOneAndDelete collection method.
func (c *Collection) FindOneAndDelete(ctx context.Context, filter interface{}, opts ...*options.FindOneAndDeleteOptions) lungo.ISingleResult {
	// trace
	ctx, span := xo.Trace(ctx, "coal/Collection.FindOneAndDelete")
	span.Tag("collection", c.coll.Name())
	defer span.End()

	// find one and delete
	res := c.coll.FindOneAndDelete(ctx, filter, opts...)

	return &SingleResult{res: res}
}

// FindOneAndReplace wraps the native FindOneAndReplace collection method.
func (c *Collection) FindOneAndReplace(ctx context.Context, filter interface{}, replacement interface{}, opts ...*options.FindOneAndReplaceOptions) lungo.ISingleResult {
	// trace
	ctx, span := xo.Trace(ctx, "coal/Collection.FindOneAndReplace")
	span.Tag("collection", c.coll.Name())
	defer span.End()

	// find and replace one
	res := c.coll.FindOneAndReplace(ctx, filter, replacement, opts...)

	return &SingleResult{res: res}
}

// FindOneAndUpdate wraps the native FindOneAndUpdate collection method.
func (c *Collection) FindOneAndUpdate(ctx context.Context, filter interface{}, update interface{}, opts ...*options.FindOneAndUpdateOptions) lungo.ISingleResult {
	// trace
	ctx, span := xo.Trace(ctx, "coal/Collection.FindOneAndUpdate")
	span.Tag("collection", c.coll.Name())
	defer span.End()

	// find one and update
	res := c.coll.FindOneAndUpdate(ctx, filter, update, opts...)

	return &SingleResult{res: res}
}

// InsertMany wraps the native InsertMany collection method.
func (c *Collection) InsertMany(ctx context.Context, documents []interface{}, opts ...*options.InsertManyOptions) (*mongo.InsertManyResult, error) {
	// trace
	ctx, span := xo.Trace(ctx, "coal/Collection.InsertMany")
	span.Tag("collection", c.coll.Name())
	span.Tag("count", len(documents))
	defer span.End()

	// insert many
	res, err := c.coll.InsertMany(ctx, documents, opts...)
	if err != nil {
		return nil, xo.W(err)
	}

	return res, nil
}

// InsertOne wraps the native InsertOne collection method.
func (c *Collection) InsertOne(ctx context.Context, document interface{}, opts ...*options.InsertOneOptions) (*mongo.InsertOneResult, error) {
	// trace
	ctx, span := xo.Trace(ctx, "coal/Collection.InsertOne")
	span.Tag("collection", c.coll.Name())
	defer span.End()

	// insert one
	res, err := c.coll.InsertOne(ctx, document, opts...)
	if err != nil {
		return nil, xo.W(err)
	}

	return res, nil
}

// ReplaceOne wraps the native ReplaceOne collection method.
func (c *Collection) ReplaceOne(ctx context.Context, filter interface{}, replacement interface{}, opts ...*options.ReplaceOptions) (*mongo.UpdateResult, error) {
	// trace
	ctx, span := xo.Trace(ctx, "coal/Collection.ReplaceOne")
	span.Tag("collection", c.coll.Name())
	defer span.End()

	// replace one
	res, err := c.coll.ReplaceOne(ctx, filter, replacement, opts...)
	if err != nil {
		return nil, xo.W(err)
	}

	return res, nil
}

// UpdateMany wraps the native UpdateMany collection method.
func (c *Collection) UpdateMany(ctx context.Context, filter interface{}, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
	// trace
	ctx, span := xo.Trace(ctx, "coal/Collection.UpdateMany")
	span.Tag("collection", c.coll.Name())
	defer span.End()

	// update many
	res, err := c.coll.UpdateMany(ctx, filter, update, opts...)
	if err != nil {
		return nil, xo.W(err)
	}

	// log result
	span.Tag("matched", res.MatchedCount)
	span.Tag("modified", res.ModifiedCount)
	span.Tag("upserted", res.UpsertedCount)

	return res, nil
}

// UpdateOne wraps the native UpdateOne collection method.
func (c *Collection) UpdateOne(ctx context.Context, filter interface{}, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
	// trace
	ctx, span := xo.Trace(ctx, "coal/Collection.UpdateOne")
	span.Tag("collection", c.coll.Name())
	defer span.End()

	// update one
	res, err := c.coll.UpdateOne(ctx, filter, update, opts...)
	if err != nil {
		return nil, xo.W(err)
	}

	// log result
	span.Tag("matched", res.MatchedCount == 1)
	span.Tag("modified", res.ModifiedCount == 1)
	span.Tag("upserted", res.UpsertedCount == 1)

	return res, nil
}

// Iterator manages the iteration over a cursor.
type Iterator struct {
	ctx     context.Context
	cursor  lungo.ICursor
	spans   []xo.Span
	counter int64
	error   error
}

func newIterator(ctx context.Context, cursor lungo.ICursor, span xo.Span) *Iterator {
	return &Iterator{
		ctx:    ctx,
		cursor: cursor,
		spans:  []xo.Span{span},
	}
}

// All will load all documents from the cursor and add them to the provided list.
// If the cursor is exhausted or an error occurred the cursor is closed.
func (i *Iterator) All(list interface{}) error {
	// get initial length
	length := int64(reflect.ValueOf(list).Elem().Len())

	// decode all documents
	err := i.cursor.All(i.ctx, list)

	// set counter
	i.counter = int64(reflect.ValueOf(list).Elem().Len()) - length

	// finish spans
	for _, span := range i.spans {
		span.Tag("loaded", i.counter)
		span.End()
	}

	return xo.W(err)
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
	return xo.W(i.cursor.Decode(v))
}

// Error returns the first error encountered during iteration. It should always
// be checked when finished to ensure there have been no errors.
func (i *Iterator) Error() error {
	return xo.W(i.error)
}

// Close will close the underlying cursor. A call to it should be deferred right
// after obtaining an iterator. Close should be called also if the iterator is
// still valid but no longer used by the application.
func (i *Iterator) Close() {
	// close cursor if available
	if i.cursor != nil {
		_ = i.cursor.Close(i.ctx)
	}

	// finish spans
	for _, span := range i.spans {
		span.Tag("loaded", i.counter)
		span.End()
	}

	// unset spans
	i.spans = nil
}

// SingleResult wraps a single operation result.
type SingleResult struct {
	res lungo.ISingleResult
}

// Decode will decode the document to the specified value.
func (r *SingleResult) Decode(i interface{}) error {
	return xo.W(r.res.Decode(i))
}

// DecodeBytes will return the raw document bytes.s
func (r *SingleResult) DecodeBytes() (bson.Raw, error) {
	raw, err := r.res.DecodeBytes()
	return raw, xo.W(err)
}

// Err return will return the operations error.
func (r *SingleResult) Err() error {
	return xo.W(r.res.Err())
}
