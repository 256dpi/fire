package coal

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/256dpi/fire/cinder"
)

// Manager manages a collection of documents.
type Manager struct {
	meta  *Meta
	coll  *Collection
	trans *Translator
}

// Find will find the document with the specified id. It will return whether
// a document has been found.
func (m *Manager) Find(ctx context.Context, model Model, id ID) (bool, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.Find")
	span.Log("id", id.Hex())
	defer span.Finish()

	// load model
	err := m.coll.FindOne(ctx, bson.M{
		"_id": id,
	}).Decode(model)
	if IsMissing(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}

// FindFirst will find the first document that matches the specified query. It
// will return whether a document has been found.
func (m *Manager) FindFirst(ctx context.Context, model Model, query bson.M, sort []string, skip int64) (bool, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.FindFirst")
	span.Log("query", query)
	span.Log("sort", sort)
	span.Log("skip", skip)
	defer span.Finish()

	// translate query
	queryDoc, err := m.trans.Document(query)
	if err != nil {
		return false, err
	}

	// prepare options
	opts := options.FindOne()

	// set sort
	if len(sort) > 0 {
		sortDoc, err := m.trans.Sort(sort)
		if err != nil {
			return false, err
		}

		opts.SetSort(sortDoc)
	}

	// set skip
	if skip > 0 {
		opts.SetSkip(skip)
	}

	// load model
	err = m.coll.FindOne(ctx, queryDoc, opts).Decode(model)
	if IsMissing(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}

// FindAll will find all documents that match the specified query.
func (m *Manager) FindAll(ctx context.Context, slicePtr interface{}, query bson.M, sort []string, skip, limit int64) error {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.FindAll")
	span.Log("query", query)
	span.Log("sort", sort)
	span.Log("skip", skip)
	span.Log("limit", limit)
	defer span.Finish()

	// translate query
	queryDoc, err := m.trans.Document(query)
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

	// find all
	err = m.coll.FindAll(ctx, slicePtr, queryDoc, opts)
	if err != nil {
		return err
	}

	return nil
}

// FindEach will find all documents that match the specified query.
func (m *Manager) FindEach(ctx context.Context, query bson.M, sort []string, skip, limit int64) (*Iterator, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.FindEach")
	span.Log("query", query)
	span.Log("sort", sort)
	span.Log("skip", skip)
	span.Log("limit", limit)

	// translate query
	queryDoc, err := m.trans.Document(query)
	if err != nil {
		span.Finish()
		return nil, err
	}

	// prepare options
	opts := options.Find()

	// set sort
	if len(sort) > 0 {
		sortDoc, err := m.trans.Sort(sort)
		if err != nil {
			span.Finish()
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

	// find documents
	iter, err := m.coll.Find(ctx, queryDoc, opts)
	if err != nil {
		span.Finish()
		return nil, err
	}

	// attach span
	iter.spans = append(iter.spans, span)

	return iter, nil
}

// Count will count the documents that match the specified query. The document
// count is estimated to avoid a full collection scan if no query, skip and
// limit is specified
func (m *Manager) Count(ctx context.Context, query bson.M, skip, limit int64) (int64, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.Count")
	span.Log("query", query)
	span.Log("skip", skip)
	span.Log("limit", limit)
	defer span.Finish()

	// estimate document count if no query and options are specified
	if len(query) == 0 && skip == 0 && limit == 0 {
		count, err := m.coll.EstimatedDocumentCount(ctx)
		if err != nil {
			return 0, err
		}

		return count, nil
	}

	// translate query
	queryDoc, err := m.trans.Document(query)
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

	// count documents
	count, err := m.coll.CountDocuments(ctx, queryDoc, opts)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// Insert will insert the provided document. If the document has a zero id a new
// id will be generated and assigned. It will return whether a document has been
// inserted.
func (m *Manager) Insert(ctx context.Context, model Model) error {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.Insert")
	defer span.Finish()

	// ensure id
	if model.ID().IsZero() {
		model.GetBase().DocID = New()
	}

	// insert document
	_, err := m.coll.InsertOne(ctx, model)
	if err != nil {
		return err
	}

	return nil
}

// InsertIfMissing will insert the provided document if no document matched the
// provided query. If the document has a zero id a new id will be generated and
// assigned. It will return whether a document has been inserted. The underlying
// upsert operation will merge the query with the model fields.
func (m *Manager) InsertIfMissing(ctx context.Context, query bson.M, model Model) (bool, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.InsertIfMissing")
	span.Log("query", query)
	defer span.Finish()

	// translate query
	queryDoc, err := m.trans.Document(query)
	if err != nil {
		return false, err
	}

	// ensure id
	if model.ID().IsZero() {
		model.GetBase().DocID = New()
	}

	// prepare options
	opts := options.Update().SetUpsert(true)

	// upsert document
	res, err := m.coll.UpdateOne(ctx, queryDoc, bson.M{
		"$setOnInsert": model,
	}, opts)
	if err != nil {
		return false, err
	}

	return res.UpsertedCount == 1, nil
}

// Replace will replace the existing document with the provided one.
func (m *Manager) Replace(ctx context.Context, model Model) error {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.Replace")
	defer span.Finish()

	// check id
	if model.ID().IsZero() {
		return fmt.Errorf("model has a zero id")
	}

	// replace document
	_, err := m.coll.ReplaceOne(ctx, bson.M{
		"_id": model.ID(),
	}, model)
	if err != nil {
		return err
	}

	return nil
}

// Update will update the document with the specified id. It will return whether
// a document has been found and updated.
func (m *Manager) Update(ctx context.Context, id ID, update bson.M) (bool, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.Update")
	span.Log("id", id.Hex())
	span.Log("update", update)
	defer span.Finish()

	// translate update
	updateDoc, err := m.trans.Document(update)
	if err != nil {
		return false, err
	}

	// update document
	res, err := m.coll.UpdateOne(ctx, bson.M{
		"_id": id,
	}, updateDoc)
	if err != nil {
		return false, err
	}

	return res.ModifiedCount == 1, nil
}

// UpdateFirst will update the first document that matches the specified query.
// It will return whether a document has been found and updated.
func (m *Manager) UpdateFirst(ctx context.Context, query, update bson.M) (bool, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.UpdateFirst")
	span.Log("query", query)
	span.Log("update", update)
	defer span.Finish()

	// translate query
	queryDoc, err := m.trans.Document(query)
	if err != nil {
		return false, err
	}

	// translate update
	updateDoc, err := m.trans.Document(update)
	if err != nil {
		return false, err
	}

	// update document
	res, err := m.coll.UpdateOne(ctx, queryDoc, updateDoc)
	if err != nil {
		return false, err
	}

	return res.ModifiedCount == 1, nil
}

// UpdateAll will update the documents that match the specified query. It will
// return the number of updated documents.
func (m *Manager) UpdateAll(ctx context.Context, query, update bson.M) (int64, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.UpdateAll")
	span.Log("query", query)
	span.Log("update", update)
	defer span.Finish()

	// translate query
	queryDoc, err := m.trans.Document(query)
	if err != nil {
		return 0, err
	}

	// translate update
	updateDoc, err := m.trans.Document(update)
	if err != nil {
		return 0, err
	}

	// update documents
	res, err := m.coll.UpdateMany(ctx, queryDoc, updateDoc)
	if err != nil {
		return 0, err
	}

	return res.ModifiedCount, nil
}

// Upsert will update the first document that matches the specified query. If
// not document has been found, the update document is applied to the query and
// inserted. It will return whether a document has been inserted.
func (m *Manager) Upsert(ctx context.Context, query, update bson.M) (bool, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.Upsert")
	span.Log("query", query)
	span.Log("update", update)
	defer span.Finish()

	// translate query
	queryDoc, err := m.trans.Document(query)
	if err != nil {
		return false, err
	}

	// translate update
	updateDoc, err := m.trans.Document(update)
	if err != nil {
		return false, err
	}

	// prepare options
	opts := options.Update().SetUpsert(true)

	// update document
	res, err := m.coll.UpdateOne(ctx, queryDoc, updateDoc, opts)
	if err != nil {
		return false, err
	}

	return res.UpsertedCount == 1, nil
}

// Delete will remove the document with the specified id. It will return
// whether a document has been found and deleted.
func (m *Manager) Delete(ctx context.Context, id ID) (bool, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.Delete")
	span.Log("id", id.Hex())
	defer span.Finish()

	// delete document
	res, err := m.coll.DeleteOne(ctx, bson.M{
		"_id": id,
	})
	if err != nil {
		return false, err
	}

	return res.DeletedCount == 1, nil
}

// DeleteAll will delete the documents that match the specified query. It will
// return the number of deleted documents.
func (m *Manager) DeleteAll(ctx context.Context, query bson.M) (int64, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.DeleteAll")
	span.Log("query", query)
	defer span.Finish()

	// translate query
	queryDoc, err := m.trans.Document(query)
	if err != nil {
		return 0, err
	}

	// update documents
	res, err := m.coll.DeleteMany(ctx, queryDoc)
	if err != nil {
		return 0, err
	}

	return res.DeletedCount, nil
}

// DeleteFirst will delete the first document that matches the specified query.
// It will return whether a document has been deleted.
func (m *Manager) DeleteFirst(ctx context.Context, query bson.M) (bool, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.DeleteFirst")
	span.Log("query", query)
	defer span.Finish()

	// translate query
	queryDoc, err := m.trans.Document(query)
	if err != nil {
		return false, err
	}

	// update document
	res, err := m.coll.DeleteOne(ctx, queryDoc)
	if err != nil {
		return false, err
	}

	return res.DeletedCount == 1, nil
}
