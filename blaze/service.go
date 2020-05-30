package blaze

import (
	"context"

	"github.com/256dpi/fire/stick"
)

// ErrInvalidHandle is returned if the provided handle is invalid.
var ErrInvalidHandle = stick.F("invalid handle")

// ErrUsedHandle is returned if the provided handle has already been used.
var ErrUsedHandle = stick.F("used handle")

// ErrNotFound is returned if there is no blob for the provided handle.
var ErrNotFound = stick.F("not found")

// ErrInvalidPosition is returned if a seek resulted in an invalid position.
var ErrInvalidPosition = stick.F("invalid position")

// Handle is a reference to a blob stored in a service.
type Handle map[string]interface{}

// Upload handles the upload of a blob.
type Upload interface {
	// Resume() (int64, error)
	Write(data []byte) (int, error)
	// Suspend() (int64, error)
	Abort() error
	Close() error
}

// Download handles the download of a blob.
type Download interface {
	Seek(offset int64, whence int) (int64, error)
	Read(buf []byte) (int, error)
	Close() error
}

// Service is responsible for managing blobs.
type Service interface {
	// Prepare should return a new handle for uploading a blob.
	Prepare(ctx context.Context) (Handle, error)

	// Upload should initiate the upload of a blob.
	Upload(ctx context.Context, handle Handle, mediaType string) (Upload, error)

	// Download should initiate the download of a blob.
	Download(ctx context.Context, handle Handle) (Download, error)

	// Delete should delete the blob.
	Delete(ctx context.Context, handle Handle) (bool, error)

	// Cleanup is called periodically and allows the service to cleanup its
	// storage until the context is cancelled.
	Cleanup(ctx context.Context) error
}
