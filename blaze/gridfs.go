package blaze

import (
	"context"

	"github.com/256dpi/lungo"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/256dpi/fire/coal"
)

// GridFS stores blobs in a GridFs bucket.
type GridFS struct {
	bucket *lungo.Bucket
}

// NewGridFS creates a new GridFS service.
func NewGridFS(bucket *lungo.Bucket) *GridFS {
	return &GridFS{
		bucket: bucket,
	}
}

// Initialize implements the Service interface.
func (g *GridFS) Initialize(ctx context.Context) error {
	// ensure indexes
	err := g.bucket.EnsureIndexes(ctx, false)
	if err != nil {
		return err
	}

	return nil
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
func (g *GridFS) Upload(ctx context.Context, handle Handle, _ string) (Upload, error) {
	// get id
	id, ok := handle["id"].(primitive.ObjectID)
	if !ok || id.IsZero() {
		return nil, ErrInvalidHandle
	}

	// open stream
	stream, err := g.bucket.OpenUploadStreamWithID(ctx, id, "")
	if err != nil {
		return nil, err
	}

	return &gridFSUpload{
		stream: stream,
	}, nil
}

// Download implements the Service interface.
func (g *GridFS) Download(ctx context.Context, handle Handle) (Download, error) {
	// get id
	id, ok := handle["id"].(primitive.ObjectID)
	if !ok || id.IsZero() {
		return nil, ErrInvalidHandle
	}

	// open download stream
	stream, err := g.bucket.OpenDownloadStream(ctx, id)
	if err == lungo.ErrFileNotFound {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}

	return &gridFSDownload{
		stream: stream,
	}, nil
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

type gridFSUpload struct {
	stream *lungo.UploadStream
}

func (u *gridFSUpload) Resume() (int64, error) {
	panic("implement me")
}

func (u *gridFSUpload) Write(data []byte) (int, error) {
	// write stream
	n, err := u.stream.Write(data)
	if coal.IsDuplicate(err) {
		return 0, ErrUsedHandle
	} else if err != nil {
		return 0, err
	}

	return n, nil
}

func (u *gridFSUpload) Suspend() (int64, error) {
	panic("implement me")
}

func (u *gridFSUpload) Abort() error {
	panic("implement me")
}

func (u *gridFSUpload) Close() error {
	// close stream
	err := u.stream.Close()
	if coal.IsDuplicate(err) {
		return ErrUsedHandle
	} else if err != nil {
		return err
	}

	return nil
}

type gridFSDownload struct {
	stream *lungo.DownloadStream
}

func (d *gridFSDownload) Seek(offset int64, whence int) (int64, error) {
	panic("implement me")
}

func (d *gridFSDownload) Read(buf []byte) (int, error) {
	// read stream
	n, err := d.stream.Read(buf)
	if err == lungo.ErrFileNotFound {
		return 0, ErrNotFound
	} else if err != nil {
		return 0, err
	}

	return n, nil
}

func (d *gridFSDownload) Close() error {
	// close stream
	err := d.stream.Close()
	if err == lungo.ErrFileNotFound {
		return ErrNotFound
	} else if err != nil {
		return err
	}

	return nil
}
