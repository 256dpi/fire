package blaze

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/256dpi/xo"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/256dpi/fire/coal"
)

// Minio stores blobs in a S3 compatible bucket.
type Minio struct {
	client *minio.Client
	bucket string
}

// NewMinioURL creates a new Minio service from a URL in the format
// `https://key:secret@host/bucket`.
func NewMinioURL(str string) (*Minio, error) {
	// parse URL
	uri, err := url.Parse(str)
	if err != nil {
		return nil, err
	}

	// check host
	if uri.Host == "" {
		return nil, xo.F("missing host")
	}

	// get bucket
	bucket := strings.Trim(uri.Path, "/")

	// get key and secret
	key := uri.User.Username()
	secret, ok := uri.User.Password()
	if !ok {
		return nil, xo.F("missing secret")
	}

	// get client
	client, err := minio.New(uri.Host, &minio.Options{
		Creds:  credentials.NewStaticV4(key, secret, ""),
		Secure: uri.Scheme == "https",
	})
	if err != nil {
		return nil, err
	}

	return NewMinio(client, bucket), nil
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
func (m *Minio) Upload(ctx context.Context, handle Handle, info Info) (Upload, error) {
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
		_, err := m.client.PutObject(ctx, m.bucket, name, upload, info.Size, minio.PutObjectOptions{
			ContentType: info.MediaType,
		})
		return err
	})

	return upload, nil
}

// Lookup implements the Service interface.
func (m *Minio) Lookup(ctx context.Context, handle Handle) (Info, error) {
	// ensure context
	if ctx == nil {
		ctx = context.Background()
	}

	// get name
	name, ok := handle["name"].(string)
	if !ok || name == "" {
		return Info{}, ErrInvalidHandle.Wrap()
	}

	// check object
	info, err := m.client.StatObject(ctx, m.bucket, name, minio.StatObjectOptions{})
	if isMinioNotFoundErr(err) {
		return Info{}, ErrNotFound.Wrap()
	} else if err != nil {
		return Info{}, err
	}

	return Info{
		Size:      info.Size,
		MediaType: info.ContentType,
	}, nil
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

	// check object
	info, err := m.client.StatObject(ctx, m.bucket, name, minio.StatObjectOptions{})
	if isMinioNotFoundErr(err) {
		return nil, ErrNotFound.Wrap()
	} else if err != nil {
		return nil, err
	}

	// prepare download
	download := SeekableDownload(info.Size, func(offset int64) (io.ReadCloser, error) {
		// prepare options
		opts := minio.GetObjectOptions{}

		// set range
		if offset > 0 {
			err = opts.SetRange(offset, 0)
			if err != nil {
				return nil, err
			}
		}

		// get object
		obj, err := m.client.GetObject(ctx, m.bucket, name, opts)
		if err != nil {
			return nil, err
		}

		return obj, nil
	})

	return download, nil
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

func isMinioNotFoundErr(err error) bool {
	return minio.ToErrorResponse(err).StatusCode == http.StatusNotFound
}
