package coal

import (
	"context"
	"errors"

	"github.com/256dpi/lungo"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/256dpi/fire/cinder"
)

// ErrBreak can be returned to break out from an iterator.
var ErrBreak = errors.New("break")

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
	coll  lungo.ICollection
	trace *cinder.Trace
}

// Aggregate wraps the native Aggregate collection method and yields the
// returned cursor.
func (c *Collection) Aggregate(ctx context.Context, pipeline interface{}, fn func(lungo.ICursor) error, opts ...*options.AggregateOptions) error {
	// trace
	if c.trace != nil {
		c.trace.Push("coal/Collection.Aggregate")
		c.trace.Tag("pipeline", pipeline)
		defer c.trace.Pop()
	}

	// run query
	csr, err := c.coll.Aggregate(ctx, pipeline, opts...)
	if err != nil {
		return err
	}

	// yield cursor
	err = fn(csr)
	if err != nil {
		_ = csr.Close(ctx)
		return err
	}

	// close cursor
	err = csr.Close(ctx)
	if err != nil {
		return err
	}

	return nil
}

// AggregateAll wraps the native Aggregate collection method and decodes all
// documents to the provided slice.
func (c *Collection) AggregateAll(ctx context.Context, slicePtr interface{}, pipeline interface{}, opts ...*options.AggregateOptions) error {
	// trace
	if c.trace != nil {
		c.trace.Push("coal/Collection.AggregateAll")
		c.trace.Tag("pipeline", pipeline)
		defer c.trace.Pop()
	}

	// run query
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

	return nil
}

// AggregateIter wraps the native Aggregate collection method and calls the
// provided callback with the decode method until ErrBreak an error is returned
// or the cursor has been exhausted.
func (c *Collection) AggregateIter(ctx context.Context, pipeline interface{}, fn func(func(interface{}) error) error, opts ...*options.AggregateOptions) error {
	// trace
	if c.trace != nil {
		c.trace.Push("coal/Collection.AggregateIter")
		c.trace.Tag("pipeline", pipeline)
		defer c.trace.Pop()
	}

	// run query
	csr, err := c.coll.Aggregate(ctx, pipeline, opts...)
	if err != nil {
		return err
	}

	// iterate over all documents
	for csr.Next(ctx) {
		err = fn(csr.Decode)
		if err == ErrBreak {
			break
		} else if err != nil {
			_ = csr.Close(ctx)
			return err
		}
	}

	// close cursor
	err = csr.Close(ctx)
	if err != nil {
		return err
	}

	return nil
}

// BulkWrite wraps the native BulkWrite collection method.
func (c *Collection) BulkWrite(ctx context.Context, models []mongo.WriteModel, opts ...*options.BulkWriteOptions) (*mongo.BulkWriteResult, error) {
	// trace
	if c.trace != nil {
		c.trace.Push("coal/Collection.BulkWrite")
		defer c.trace.Pop()
	}

	return c.coll.BulkWrite(ctx, models, opts...)
}

// CountDocuments wraps the native CountDocuments collection method.
func (c *Collection) CountDocuments(ctx context.Context, filter interface{}, opts ...*options.CountOptions) (int64, error) {
	// trace
	if c.trace != nil {
		c.trace.Push("coal/Collection.CountDocuments")
		c.trace.Log("filter", filter)
		defer c.trace.Pop()
	}

	return c.coll.CountDocuments(ctx, filter, opts...)
}

// DeleteMany wraps the native DeleteMany collection method.
func (c *Collection) DeleteMany(ctx context.Context, filter interface{}, opts ...*options.DeleteOptions) (*mongo.DeleteResult, error) {
	// trace
	if c.trace != nil {
		// push span
		c.trace.Push("coal/Collection.DeleteMany")
		c.trace.Log("filter", filter)
		defer c.trace.Pop()
	}

	return c.coll.DeleteMany(ctx, filter, opts...)
}

// DeleteOne wraps the native DeleteOne collection method.
func (c *Collection) DeleteOne(ctx context.Context, filter interface{}, opts ...*options.DeleteOptions) (*mongo.DeleteResult, error) {
	// trace
	if c.trace != nil {
		c.trace.Push("coal/Collection.DeleteOne")
		c.trace.Log("filter", filter)
		defer c.trace.Pop()
	}

	return c.coll.DeleteOne(ctx, filter, opts...)
}

// Distinct wraps the native Distinct collection method.
func (c *Collection) Distinct(ctx context.Context, field string, filter interface{}, opts ...*options.DistinctOptions) ([]interface{}, error) {
	// trace
	if c.trace != nil {
		c.trace.Push("coal/Collection.Distinct")
		c.trace.Tag("field", field)
		c.trace.Log("filter", filter)
		defer c.trace.Pop()
	}

	return c.coll.Distinct(ctx, field, filter, opts...)
}

// EstimatedDocumentCount wraps the native EstimatedDocumentCount collection method.
func (c *Collection) EstimatedDocumentCount(ctx context.Context, opts ...*options.EstimatedDocumentCountOptions) (int64, error) {
	// trace
	if c.trace != nil {
		c.trace.Push("coal/Collection.EstimatedDocumentCount")
		defer c.trace.Pop()
	}

	return c.coll.EstimatedDocumentCount(ctx, opts...)
}

// FindAll wraps the native Find collection method and yields the returned cursor.
func (c *Collection) Find(ctx context.Context, filter interface{}, fn func(csr lungo.ICursor) error, opts ...*options.FindOptions) error {
	// trace
	if c.trace != nil {
		c.trace.Push("coal/Collection.Find")
		c.trace.Tag("filter", filter)
		defer c.trace.Pop()
	}

	// run query
	csr, err := c.coll.Find(ctx, filter, opts...)
	if err != nil {
		return err
	}

	// yield cursor
	err = fn(csr)
	if err != nil {
		_ = csr.Close(ctx)
		return err
	}

	// close cursor
	err = csr.Close(ctx)
	if err != nil {
		return err
	}

	return nil
}

// FindAll wraps the native Find collection method and decodes all documents to
// the provided slice.
func (c *Collection) FindAll(ctx context.Context, slicePtr interface{}, filter interface{}, opts ...*options.FindOptions) error {
	// trace
	if c.trace != nil {
		c.trace.Push("coal/Collection.FindAll")
		c.trace.Tag("filter", filter)
		defer c.trace.Pop()
	}

	// run query
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

	return nil
}

// FindIter wraps the native Find collection method and calls the provided
// callback with the decode method until ErrBreak or an error is returned or the
// cursor has been exhausted.
func (c *Collection) FindIter(ctx context.Context, filter interface{}, fn func(func(interface{}) error) error, opts ...*options.FindOptions) error {
	// trace
	if c.trace != nil {
		c.trace.Push("coal/Collection.FindIter")
		c.trace.Tag("filter", filter)
		defer c.trace.Pop()
	}

	// run query
	csr, err := c.coll.Find(ctx, filter, opts...)
	if err != nil {
		return err
	}

	// iterate over all documents
	for csr.Next(ctx) {
		err = fn(csr.Decode)
		if err == ErrBreak {
			break
		} else if err != nil {
			_ = csr.Close(ctx)
			return err
		}
	}

	// close cursor
	err = csr.Close(ctx)
	if err != nil {
		return err
	}

	return nil
}

// FindOne wraps the native FindOne collection method.
func (c *Collection) FindOne(ctx context.Context, filter interface{}, opts ...*options.FindOneOptions) lungo.ISingleResult {
	// trace
	if c.trace != nil {
		c.trace.Push("coal/Collection.FindOne")
		c.trace.Log("filter", filter)
		defer c.trace.Pop()
	}

	return c.coll.FindOne(ctx, filter, opts...)
}

// FindOneAndDelete wraps the native FindOneAndDelete collection method.
func (c *Collection) FindOneAndDelete(ctx context.Context, filter interface{}, opts ...*options.FindOneAndDeleteOptions) lungo.ISingleResult {
	// trace
	if c.trace != nil {
		c.trace.Push("coal/Collection.FindOneAndDelete")
		c.trace.Log("filter", filter)
		defer c.trace.Pop()
	}

	return c.coll.FindOneAndDelete(ctx, filter, opts...)
}

// FindOneAndReplace wraps the native FindOneAndReplace collection method.
func (c *Collection) FindOneAndReplace(ctx context.Context, filter interface{}, replacement interface{}, opts ...*options.FindOneAndReplaceOptions) lungo.ISingleResult {
	// trace
	if c.trace != nil {
		c.trace.Push("coal/Collection.FindOneAndReplace")
		c.trace.Log("filter", filter)
		defer c.trace.Pop()
	}

	return c.coll.FindOneAndReplace(ctx, filter, replacement, opts...)
}

// FindOneAndUpdate wraps the native FindOneAndUpdate collection method.
func (c *Collection) FindOneAndUpdate(ctx context.Context, filter interface{}, update interface{}, opts ...*options.FindOneAndUpdateOptions) lungo.ISingleResult {
	// trace
	if c.trace != nil {
		c.trace.Push("coal/Collection.FindOneAndUpdate")
		c.trace.Log("filter", filter)
		defer c.trace.Pop()
	}

	return c.coll.FindOneAndUpdate(ctx, filter, update, opts...)
}

// InsertMany wraps the native InsertMany collection method.
func (c *Collection) InsertMany(ctx context.Context, documents []interface{}, opts ...*options.InsertManyOptions) (*mongo.InsertManyResult, error) {
	// trace
	if c.trace != nil {
		c.trace.Push("coal/Collection.InsertMany")
		defer c.trace.Pop()
	}

	return c.coll.InsertMany(ctx, documents, opts...)
}

// InsertOne wraps the native InsertOne collection method.
func (c *Collection) InsertOne(ctx context.Context, document interface{}, opts ...*options.InsertOneOptions) (*mongo.InsertOneResult, error) {
	// trace
	if c.trace != nil {
		c.trace.Push("coal/Collection.InsertOne")
		defer c.trace.Pop()
	}

	return c.coll.InsertOne(ctx, document, opts...)
}

// ReplaceOne wraps the native ReplaceOne collection method.
func (c *Collection) ReplaceOne(ctx context.Context, filter interface{}, replacement interface{}, opts ...*options.ReplaceOptions) (*mongo.UpdateResult, error) {
	// trace
	if c.trace != nil {
		c.trace.Push("coal/Collection.ReplaceOne")
		c.trace.Log("filter", filter)
		defer c.trace.Pop()
	}

	return c.coll.ReplaceOne(ctx, filter, replacement, opts...)
}

// UpdateMany wraps the native UpdateMany collection method.
func (c *Collection) UpdateMany(ctx context.Context, filter interface{}, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
	// trace
	if c.trace != nil {
		c.trace.Push("coal/Collection.UpdateMany")
		c.trace.Log("filter", filter)
		defer c.trace.Pop()
	}

	return c.coll.UpdateMany(ctx, filter, update, opts...)
}

// UpdateOne wraps the native UpdateOne collection method.
func (c *Collection) UpdateOne(ctx context.Context, filter interface{}, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
	// trace
	if c.trace != nil {
		c.trace.Push("coal/Collection.UpdateOne")
		c.trace.Log("filter", filter)
		defer c.trace.Pop()
	}

	return c.coll.UpdateOne(ctx, filter, update, opts...)
}
