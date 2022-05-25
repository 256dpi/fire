package blaze

import (
	"context"
	"fmt"
	"io"
	"net/http"
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
func (m *Minio) Upload(ctx context.Context, handle Handle, name, mediaType string, size int64) (Upload, error) {
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
	if isMinioNotFoundErr(err) {
		// continue
	} else if err != nil {
		return nil, err
	} else {
		return nil, ErrUsedHandle.Wrap()
	}

	// create upload pipe
	upload := PipeUpload(func(upload io.Reader) error {
		_, err := m.client.PutObject(ctx, m.bucket, name, upload, size, minio.PutObjectOptions{
			ContentType: mediaType,
		})
		return err
	})

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
	if err != nil {
		return nil, err
	}

	// check object
	_, err = obj.Stat()
	if isMinioNotFoundErr(err) {
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
	if isMinioNotFoundErr(err) {
		return ErrNotFound.Wrap()
	} else if err != nil {
		return err
	}

	// remove object
	err = m.client.RemoveObject(ctx, m.bucket, name, minio.RemoveObjectOptions{})
	if err != nil {
		return err
	}

	return nil
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

func isMinioNotFoundErr(err error) bool {
	return minio.ToErrorResponse(err).StatusCode == http.StatusNotFound
}
