package blaze

import (
	"context"
	"io"

	"github.com/256dpi/lungo"
	"github.com/256dpi/xo"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/256dpi/fire/coal"
)

// GridFS stores blobs in a GridFS bucket.
type GridFS struct {
	bucket *lungo.Bucket
}

// NewGridFS creates a new GridFS service.
func NewGridFS(bucket *lungo.Bucket) *GridFS {
	return &GridFS{
		bucket: bucket,
	}
}

// Initialize will initialize the GridFS bucket.
func (g *GridFS) Initialize(ctx context.Context) error {
	// ensure indexes
	err := g.bucket.EnsureIndexes(ctx, false)
	if err != nil {
		return xo.W(err)
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
func (g *GridFS) Upload(ctx context.Context, handle Handle, _, _ string) (Upload, error) {
	// get id
	id, ok := handle["id"].(primitive.ObjectID)
	if !ok || id.IsZero() {
		return nil, ErrInvalidHandle.Wrap()
	}

	// open stream
	stream, err := g.bucket.OpenUploadStreamWithID(ctx, id, "")
	if err != nil {
		return nil, xo.W(err)
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
		return nil, ErrInvalidHandle.Wrap()
	}

	// open download stream
	stream, err := g.bucket.OpenDownloadStream(ctx, id)
	if err == lungo.ErrFileNotFound {
		return nil, ErrNotFound.Wrap()
	} else if err != nil {
		return nil, xo.W(err)
	}

	return &gridFSDownload{
		stream: stream,
	}, nil
}

// Delete implements the Service interface.
func (g *GridFS) Delete(ctx context.Context, handle Handle) error {
	// get id
	id, ok := handle["id"].(primitive.ObjectID)
	if !ok || id.IsZero() {
		return ErrInvalidHandle.Wrap()
	}

	// delete file
	err := g.bucket.Delete(ctx, id)
	if err != nil && err != lungo.ErrFileNotFound {
		return xo.W(err)
	}

	return nil
}

// Cleanup implements the Service interface.
func (g *GridFS) Cleanup(_ context.Context) error {
	return nil
}

type gridFSUpload struct {
	stream *lungo.UploadStream
}

// func (u *gridFSUpload) Resume() (int64, error) {
// 	panic("implement me")
// }

func (u *gridFSUpload) Write(data []byte) (int, error) {
	// write stream
	n, err := u.stream.Write(data)
	if coal.IsDuplicate(err) {
		return 0, ErrUsedHandle.Wrap()
	} else if err != nil {
		return 0, xo.W(err)
	}

	return n, nil
}

// func (u *gridFSUpload) Suspend() (int64, error) {
// 	panic("implement me")
// }

func (u *gridFSUpload) Abort() error {
	return xo.W(u.stream.Abort())
}

func (u *gridFSUpload) Close() error {
	// close stream
	err := u.stream.Close()
	if coal.IsDuplicate(err) {
		return ErrUsedHandle.Wrap()
	} else if err != nil {
		return xo.W(err)
	}

	return nil
}

type gridFSDownload struct {
	stream *lungo.DownloadStream
}

func (d *gridFSDownload) Seek(offset int64, whence int) (int64, error) {
	// seek stream
	n, err := d.stream.Seek(offset, whence)
	if err == lungo.ErrFileNotFound {
		return 0, ErrNotFound.Wrap()
	} else if err == lungo.ErrInvalidPosition {
		return 0, ErrInvalidPosition.Wrap()
	} else if err != nil {
		return 0, xo.W(err)
	}

	return n, nil
}

func (d *gridFSDownload) Read(buf []byte) (int, error) {
	// read stream
	n, err := d.stream.Read(buf)
	if err == lungo.ErrFileNotFound {
		return 0, ErrNotFound.Wrap()
	} else if err == io.EOF {
		return 0, io.EOF
	} else if err != nil {
		return 0, xo.W(err)
	}

	return n, nil
}

func (d *gridFSDownload) Close() error {
	// close stream
	err := d.stream.Close()
	if err == lungo.ErrFileNotFound {
		return ErrNotFound.Wrap()
	} else if err != nil {
		return xo.W(err)
	}

	return nil
}
