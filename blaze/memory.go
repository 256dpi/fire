package blaze

import (
	"context"
	"io"
	"io/ioutil"
	"strconv"
)

// MemoryService is a service for testing purposes that stores blobs in memory.
type MemoryService struct {
	// The stored blobs.
	Blobs map[string]Blob

	// The next id.
	Next int
}

// NewMemoryService will create a new memory service.
func NewMemoryService() *MemoryService {
	return &MemoryService{
		Blobs: map[string]Blob{},
	}
}

// Prepare implements the Service interface.
func (s *MemoryService) Prepare() (Handle, error) {
	// increment id
	s.Next++

	// create handle
	handle := Handle{
		"id": strconv.FormatInt(int64(s.Next), 10),
	}

	return handle, nil
}

// Upload implements the Service interface.
func (s *MemoryService) Upload(_ context.Context, handle Handle, contentType string, r io.Reader) (int64, error) {
	// get id
	id, _ := handle["id"].(string)
	if id == "" {
		return 0, ErrInvalidHandle
	}

	// check blob
	_, ok := s.Blobs[id]
	if ok {
		return 0, ErrUsedHandle
	}

	// read bytes
	bytes, err := ioutil.ReadAll(r)
	if err != nil {
		return 0, err
	}

	// set blob
	s.Blobs[id] = Blob{
		Type:  contentType,
		Bytes: bytes,
	}

	return int64(len(bytes)), nil
}

// Download implements the Service interface.
func (s *MemoryService) Download(_ context.Context, handle Handle, w io.Writer) error {
	// get id
	id, _ := handle["id"].(string)
	if id == "" {
		return ErrInvalidHandle
	}

	// get blob
	file, ok := s.Blobs[id]
	if !ok {
		return ErrNotFound
	}

	// write bytes
	_, err := w.Write(file.Bytes)
	if err != nil {
		return err
	}

	return nil
}

// Delete implements the Service interface.
func (s *MemoryService) Delete(_ context.Context, handle Handle) (bool, error) {
	// get id
	id, _ := handle["id"].(string)
	if id == "" {
		return false, ErrInvalidHandle
	}

	// check blob
	if _, ok := s.Blobs[id]; !ok {
		return false, ErrNotFound
	}

	// delete blob
	delete(s.Blobs, id)

	return true, nil
}

// Cleanup implements the Service interface.
func (s *MemoryService) Cleanup(_ context.Context) error {
	return nil
}
