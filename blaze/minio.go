package blaze

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/minio/minio-go/v7"

	"github.com/256dpi/fire/coal"
)

// Minio stores blobs in a S3 compatible bucket.
type Minio struct {
	client *minio.Client
	bucket string
}

// NewMinio creates a new Minio service.
func NewMinio(client *minio.Client, bucket string) *Minio {
	return &Minio{
		client: client,
		bucket: bucket,
	}
}

// Prepare implements the Service interface.
func (m *Minio) Prepare(context.Context) (Handle, error) {
	// construct name
	str := coal.New().Hex()
	name := fmt.Sprintf("%s/%s/%s", str[len(str)-2:], str[len(str)-4:len(str)-2], str)

	// create handle
	handle := Handle{
		"name": name,
	}

	return handle, nil
}

// Upload implements the Service interface.
func (m *Minio) Upload(ctx context.Context, handle Handle, name, mediaType string) (Upload, error) {
	// ensure context
	if ctx == nil {
		ctx = context.Background()
	}

	// get name
	name, ok := handle["name"].(string)
	if !ok || name == "" {
		return nil, ErrInvalidHandle.Wrap()
	}

	// check object
	_, err := m.client.StatObject(ctx, m.bucket, name, minio.StatObjectOptions{})
	if err != nil && minio.ToErrorResponse(err).Code == "NoSuchKey" {
		// good
	} else if err != nil {
		return nil, err
	} else {
		return nil, ErrUsedHandle.Wrap()
	}

	// prepare pipe
	r, w := io.Pipe()

	// prepare upload
	upload := &minioUpload{
		pipe: w,
		done: make(chan error, 1),
	}

	// start upload
	go func() {
		_, err := m.client.PutObject(ctx, m.bucket, name, r, -1, minio.PutObjectOptions{
			ContentType: mediaType,
		})
		upload.done <- err
		close(upload.done)
	}()

	return upload, nil
}

// Download implements the Service interface.
func (m *Minio) Download(ctx context.Context, handle Handle) (Download, error) {
	// ensure context
	if ctx == nil {
		ctx = context.Background()
	}

	// get name
	name, ok := handle["name"].(string)
	if !ok || name == "" {
		return nil, ErrInvalidHandle.Wrap()
	}

	// get object
	obj, err := m.client.GetObject(ctx, m.bucket, name, minio.GetObjectOptions{})
	if err != nil && minio.ToErrorResponse(err).Code == "NoSuchKey" {
		return nil, ErrNotFound.Wrap()
	} else if err != nil {
		return nil, err
	}

	// check object
	_, err = obj.Stat()
	if err != nil && minio.ToErrorResponse(err).Code == "NoSuchKey" {
		return nil, ErrNotFound.Wrap()
	} else if err != nil {
		return nil, err
	}

	return &minioDownload{Object: obj}, nil
}

// Delete implements the Service interface.
func (m *Minio) Delete(ctx context.Context, handle Handle) error {
	// ensure context
	if ctx == nil {
		ctx = context.Background()
	}

	// get name
	name, ok := handle["name"].(string)
	if !ok || name == "" {
		return ErrInvalidHandle.Wrap()
	}

	// check object
	_, err := m.client.StatObject(ctx, m.bucket, name, minio.StatObjectOptions{})
	if err != nil && minio.ToErrorResponse(err).Code == "NoSuchKey" {
		return ErrNotFound.Wrap()
	} else if err != nil {
		return err
	}

	// remove object
	err = m.client.RemoveObject(ctx, m.bucket, name, minio.RemoveObjectOptions{})
	if err != nil && minio.ToErrorResponse(err).Code == "NoSuchKey" {
		return ErrNotFound.Wrap()
	} else if err != nil {
		return err
	}

	return nil
}

var errMinioAbort = errors.New("abort")

type minioUpload struct {
	pipe *io.PipeWriter
	done chan error
}

func (p *minioUpload) Write(data []byte) (int, error) {
	return p.pipe.Write(data)
}

func (p *minioUpload) Abort() error {
	// abort upload
	err := p.pipe.CloseWithError(errMinioAbort)
	if err != nil {
		return err
	}

	return <-p.done
}

func (p *minioUpload) Close() error {
	// close upload
	err := p.pipe.Close()
	if err != nil {
		return err
	}

	return <-p.done
}

type minioDownload struct {
	*minio.Object
}

func (d *minioDownload) Seek(offset int64, whence int) (int64, error) {
	pos, err := d.Object.Seek(offset, whence)
	if err != nil && strings.Contains(minio.ToErrorResponse(err).Message, "Negative position") {
		err = ErrInvalidPosition.Wrap()
	}
	return pos, err
}
