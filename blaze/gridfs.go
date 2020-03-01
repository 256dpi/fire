package blaze

import (
	"context"
	"io"

	"github.com/256dpi/lungo"
	"github.com/256dpi/serve"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/256dpi/fire/coal"
)

// GridFS stores blobs in a GridFs bucket.
type GridFS struct {
	store  *coal.Store
	bucket *lungo.Bucket
}

// NewGridFS creates a new GridFS service.
func NewGridFS(store *coal.Store, chunkSize int64) *GridFS {
	// set default chunk size
	if chunkSize == 0 {
		chunkSize = serve.MustByteSize("2M")
	}

	return &GridFS{
		store:  store,
		bucket: lungo.NewBucket(store.DB(), options.GridFSBucket().SetChunkSizeBytes(int32(chunkSize))),
	}
}

// Prepare implements the Service interface.
func (g *GridFS) Prepare(_ context.Context) (Handle, error) {
	// create handle
	handle := Handle{
		"id": primitive.NewObjectID(),
	}

	return handle, nil
}

// Upload implements the Service interface.
func (g *GridFS) Upload(ctx context.Context, handle Handle, _ string, r io.Reader) (int64, error) {
	// get id
	id, ok := handle["id"].(primitive.ObjectID)
	if !ok || id.IsZero() {
		return 0, ErrInvalidHandle
	}

	// open stream
	stream, err := g.bucket.OpenUploadStreamWithID(ctx, id, "")
	if err != nil {
		return 0, err
	}

	// write all data
	n, err := io.Copy(stream, r)
	if err != nil {
		_ = stream.Abort()
		return 0, err
	}

	// close stream
	err = stream.Close()
	if coal.IsDuplicate(err) {
		return 0, ErrUsedHandle
	} else if err != nil {
		return 0, err
	}

	return n, nil
}

// Download implements the Service interface.
func (g *GridFS) Download(ctx context.Context, handle Handle, w io.Writer) error {
	// get id
	id, ok := handle["id"].(primitive.ObjectID)
	if !ok || id.IsZero() {
		return ErrInvalidHandle
	}

	// open download stream
	stream, err := g.bucket.OpenDownloadStream(ctx, id)
	if err == lungo.ErrFileNotFound {
		return ErrNotFound
	} else if err != nil {
		return err
	}

	// ensure stream is closed
	defer stream.Close()

	// read all data
	_, err = io.Copy(w, stream)
	if err == lungo.ErrFileNotFound {
		return ErrNotFound
	} else if err != nil {
		return err
	}

	return nil
}

// Delete implements the Service interface.
func (g *GridFS) Delete(ctx context.Context, handle Handle) (bool, error) {
	// get id
	id, ok := handle["id"].(primitive.ObjectID)
	if !ok || id.IsZero() {
		return false, ErrInvalidHandle
	}

	// delete file
	err := g.bucket.Delete(ctx, id)
	if err == lungo.ErrFileNotFound {
		return false, ErrNotFound
	} else if err != nil {
		return false, err
	}

	return true, nil
}

// Cleanup implements the Service interface.
func (g *GridFS) Cleanup(_ context.Context) error {
	return nil
}
