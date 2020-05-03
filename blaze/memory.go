package blaze

import (
	"context"
	"io"
	"strconv"
)

// Memory is a service for testing purposes that stores blobs in memory.
type Memory struct {
	// The stored blobs.
	Blobs map[string]*Blob

	// The next id.
	Next int
}

// NewMemory will create a new memory service.
func NewMemory() *Memory {
	return &Memory{
		Blobs: map[string]*Blob{},
	}
}

// Prepare implements the Service interface.
func (s *Memory) Prepare(_ context.Context) (Handle, error) {
	// increment id
	s.Next++

	// create handle
	handle := Handle{
		"id": strconv.FormatInt(int64(s.Next), 10),
	}

	return handle, nil
}

// Upload implements the Service interface.
func (s *Memory) Upload(_ context.Context, handle Handle, contentType string) (Upload, error) {
	// get id
	id, _ := handle["id"].(string)
	if id == "" {
		return nil, ErrInvalidHandle
	}

	// check blob
	_, ok := s.Blobs[id]
	if ok {
		return nil, ErrUsedHandle
	}

	// prepare blob
	blob := &Blob{
		Type: contentType,
	}

	// store blob
	s.Blobs[id] = blob

	return &memoryUpload{
		blob: blob,
	}, nil
}

// Download implements the Service interface.
func (s *Memory) Download(_ context.Context, handle Handle) (Download, error) {
	// get id
	id, _ := handle["id"].(string)
	if id == "" {
		return nil, ErrInvalidHandle
	}

	// get blob
	blob, ok := s.Blobs[id]
	if !ok {
		return nil, ErrNotFound
	}

	return &memoryDownload{
		blob: blob,
	}, nil
}

// Delete implements the Service interface.
func (s *Memory) Delete(_ context.Context, handle Handle) (bool, error) {
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
func (s *Memory) Cleanup(_ context.Context) error {
	return nil
}

type memoryUpload struct {
	blob *Blob
}

func (u *memoryUpload) Resume() (int64, error) {
	panic("implement me")
}

func (u *memoryUpload) Write(data []byte) (int, error) {
	// append data
	u.blob.Bytes = append(u.blob.Bytes, data...)

	return len(data), nil
}

func (u *memoryUpload) Suspend() (int64, error) {
	panic("implement me")
}

func (u *memoryUpload) Abort() error {
	panic("implement me")
}

func (u *memoryUpload) Close() error {
	return nil
}

type memoryDownload struct {
	blob *Blob
	pos  int
}

func (u *memoryDownload) Skip(skip int64) (int64, error) {
	panic("implement me")
}

func (u *memoryDownload) Seek(offset int64, whence int) (int64, error) {
	panic("implement me")
}

func (u *memoryDownload) Read(buf []byte) (int, error) {
	// check EOF
	if u.pos >= len(u.blob.Bytes) {
		return 0, io.EOF
	}

	// copy bytes
	n := copy(buf, u.blob.Bytes[u.pos:])
	u.pos += n

	return n, nil
}

func (u *memoryDownload) Close() error {
	return nil
}
