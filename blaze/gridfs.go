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

type GridFSService struct {
	store  *coal.Store
	bucket *lungo.Bucket
}

func NewGridFSService(store *coal.Store, chunkSize int64) *GridFSService {
	// set default chunk size
	if chunkSize == 0 {
		chunkSize = serve.MustByteSize("2M")
	}

	return &GridFSService{
		store:  store,
		bucket: lungo.NewBucket(store.DB(), options.GridFSBucket().SetChunkSizeBytes(int32(chunkSize))),
	}
}

func (g *GridFSService) Prepare() (Handle, error) {
	// create handle
	handle := Handle{
		"id": primitive.NewObjectID(),
	}

	return handle, nil
}

func (g *GridFSService) Upload(ctx context.Context, handle Handle, _ string, r io.Reader) (int64, error) {
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
	if lungo.IsUniquenessError(err) {
		return 0, ErrUsedHandle
	} else if err != nil {
		return 0, err
	}

	return n, nil
}

func (g *GridFSService) Download(ctx context.Context, handle Handle, w io.Writer) error {
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

func (g *GridFSService) Delete(ctx context.Context, handle Handle) (bool, error) {
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

func (g *GridFSService) Cleanup(_ context.Context) error {
	return nil
}
