package coal

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Tracer is used by a traced collection to push tracing spans for database
// queries.
type Tracer interface {
	Push(name string)
	Tag(key string, value interface{})
	Log(key string, value interface{})
	Pop()
}

// TracedCollection wraps a collection to automatically push tracing spans for
// run queries.
type TracedCollection struct {
	coll   *mongo.Collection
	tracer Tracer
}

// AggregateAll wraps the native Aggregate collection method and decodes all
// documents to the provided slice.
func (c *TracedCollection) AggregateAll(ctx context.Context, slicePtr interface{}, pipeline interface{}, opts ...*options.AggregateOptions) error {
	// push span
	c.tracer.Push("mongo/Collection.Aggregate")
	c.tracer.Tag("pipeline", pipeline)
	defer c.tracer.Pop()

	// run query
	csr, err := c.coll.Aggregate(ctx, pipeline, opts...)
	if err != nil {
		return err
	}

	// decode all documents
	err = csr.All(ctx, slicePtr)
	if err != nil {
		return err
	}

	// close cursor
	err = csr.Close(ctx)
	if err != nil {
		return err
	}

	return nil
}

// BulkWrite wraps the native BulkWrite collection method.
func (c *TracedCollection) BulkWrite(ctx context.Context, models []mongo.WriteModel, opts ...*options.BulkWriteOptions) (*mongo.BulkWriteResult, error) {
	// push span
	c.tracer.Push("mongo/Collection.BulkWrite")
	defer c.tracer.Pop()

	// run query
	return c.coll.BulkWrite(ctx, models, opts...)
}

// CountDocuments wraps the native CountDocuments collection method.
func (c *TracedCollection) CountDocuments(ctx context.Context, filter interface{}, opts ...*options.CountOptions) (int64, error) {
	// push span
	c.tracer.Push("mongo/Collection.CountDocuments")
	c.tracer.Log("filter", filter)
	defer c.tracer.Pop()

	// run query
	return c.coll.CountDocuments(ctx, filter, opts...)
}

// DeleteMany wraps the native DeleteMany collection method.
func (c *TracedCollection) DeleteMany(ctx context.Context, filter interface{}, opts ...*options.DeleteOptions) (*mongo.DeleteResult, error) {
	// push span
	c.tracer.Push("mongo/Collection.DeleteMany")
	c.tracer.Log("filter", filter)
	defer c.tracer.Pop()

	// run query
	return c.coll.DeleteMany(ctx, filter, opts...)
}

// DeleteOne wraps the native DeleteOne collection method.
func (c *TracedCollection) DeleteOne(ctx context.Context, filter interface{}, opts ...*options.DeleteOptions) (*mongo.DeleteResult, error) {
	// push span
	c.tracer.Push("mongo/Collection.DeleteOne")
	c.tracer.Log("filter", filter)
	defer c.tracer.Pop()

	// run query
	return c.coll.DeleteOne(ctx, filter, opts...)
}

// Distinct wraps the native Distinct collection method.
func (c *TracedCollection) Distinct(ctx context.Context, fieldName string, filter interface{}, opts ...*options.DistinctOptions) ([]interface{}, error) {
	// push span
	c.tracer.Push("mongo/Collection.Distinct")
	c.tracer.Tag("fieldName", fieldName)
	c.tracer.Log("filter", filter)
	defer c.tracer.Pop()

	// run query
	return c.coll.Distinct(ctx, fieldName, filter, opts...)
}

// EstimatedDocumentCount wraps the native EstimatedDocumentCount collection method.
func (c *TracedCollection) EstimatedDocumentCount(ctx context.Context, opts ...*options.EstimatedDocumentCountOptions) (int64, error) {
	// push span
	c.tracer.Push("mongo/Collection.EstimatedDocumentCount")
	defer c.tracer.Pop()

	// run query
	return c.coll.EstimatedDocumentCount(ctx, opts...)
}

// FindAll wraps the native Find collection method and decodes all documents to
// the provided slice.
func (c *TracedCollection) FindAll(ctx context.Context, slicePtr interface{}, filter interface{}, opts ...*options.FindOptions) error {
	// push span
	c.tracer.Push("mongo/Collection.Find")
	c.tracer.Tag("filter", filter)
	defer c.tracer.Pop()

	// run query
	csr, err := c.coll.Find(ctx, filter, opts...)
	if err != nil {
		return err
	}

	// decode all documents
	err = csr.All(ctx, slicePtr)
	if err != nil {
		return err
	}

	// close cursor
	err = csr.Close(ctx)
	if err != nil {
		return err
	}

	return nil
}

// FindOne wraps the native FindOne collection method.
func (c *TracedCollection) FindOne(ctx context.Context, filter interface{}, opts ...*options.FindOneOptions) *mongo.SingleResult {
	// push span
	c.tracer.Push("mongo/Collection.FindOne")
	c.tracer.Log("filter", filter)
	defer c.tracer.Pop()

	// run query
	return c.coll.FindOne(ctx, filter, opts...)
}

// FindOneAndDelete wraps the native FindOneAndDelete collection method.
func (c *TracedCollection) FindOneAndDelete(ctx context.Context, filter interface{}, opts ...*options.FindOneAndDeleteOptions) *mongo.SingleResult {
	// push span
	c.tracer.Push("mongo/Collection.FindOneAndDelete")
	c.tracer.Log("filter", filter)
	defer c.tracer.Pop()

	// run query
	return c.coll.FindOneAndDelete(ctx, filter, opts...)
}

// FindOneAndReplace wraps the native FindOneAndReplace collection method.
func (c *TracedCollection) FindOneAndReplace(ctx context.Context, filter interface{}, replacement interface{}, opts ...*options.FindOneAndReplaceOptions) *mongo.SingleResult {
	// push span
	c.tracer.Push("mongo/Collection.FindOneAndReplace")
	c.tracer.Log("filter", filter)
	defer c.tracer.Pop()

	// run query
	return c.coll.FindOneAndReplace(ctx, filter, replacement, opts...)
}

// FindOneAndUpdate wraps the native FindOneAndUpdate collection method.
func (c *TracedCollection) FindOneAndUpdate(ctx context.Context, filter interface{}, update interface{}, opts ...*options.FindOneAndUpdateOptions) *mongo.SingleResult {
	// push span
	c.tracer.Push("mongo/Collection.FindOneAndUpdate")
	c.tracer.Log("filter", filter)
	defer c.tracer.Pop()

	// run query
	return c.coll.FindOneAndUpdate(ctx, filter, update, opts...)
}

// InsertMany wraps the native InsertMany collection method.
func (c *TracedCollection) InsertMany(ctx context.Context, documents []interface{}, opts ...*options.InsertManyOptions) (*mongo.InsertManyResult, error) {
	// push span
	c.tracer.Push("mongo/Collection.InsertMany")
	defer c.tracer.Pop()

	// run query
	return c.coll.InsertMany(ctx, documents, opts...)
}

// InsertOne wraps the native InsertOne collection method.
func (c *TracedCollection) InsertOne(ctx context.Context, document interface{}, opts ...*options.InsertOneOptions) (*mongo.InsertOneResult, error) {
	// push span
	c.tracer.Push("mongo/Collection.InsertOne")
	defer c.tracer.Pop()

	// run query
	return c.coll.InsertOne(ctx, document, opts...)
}

// ReplaceOne wraps the native ReplaceOne collection method.
func (c *TracedCollection) ReplaceOne(ctx context.Context, filter interface{}, replacement interface{}, opts ...*options.ReplaceOptions) (*mongo.UpdateResult, error) {
	// push span
	c.tracer.Push("mongo/Collection.ReplaceOne")
	c.tracer.Log("filter", filter)
	defer c.tracer.Pop()

	// run query
	return c.coll.ReplaceOne(ctx, filter, replacement, opts...)
}

// UpdateMany wraps the native UpdateMany collection method.
func (c *TracedCollection) UpdateMany(ctx context.Context, filter interface{}, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
	// push span
	c.tracer.Push("mongo/Collection.UpdateMany")
	c.tracer.Log("filter", filter)
	defer c.tracer.Pop()

	// run query
	return c.coll.UpdateMany(ctx, filter, update, opts...)
}

// UpdateOne wraps the native UpdateOne collection method.
func (c *TracedCollection) UpdateOne(ctx context.Context, filter interface{}, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
	// push span
	c.tracer.Push("mongo/Collection.UpdateOne")
	c.tracer.Log("filter", filter)
	defer c.tracer.Pop()

	// run query
	return c.coll.UpdateOne(ctx, filter, update, opts...)
}
