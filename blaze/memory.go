package blaze

import (
	"context"
	"io"
	"strconv"
)

// MemoryBlob is a blob stored by the memory service.
type MemoryBlob struct {
	Type  string
	Bytes []byte
}

// Memory is a service for testing purposes that stores blobs in memory.
type Memory struct {
	// The stored blobs.
	Blobs map[string]*MemoryBlob

	// The next id.
	Next int
}

// NewMemory will create a new memory service.
func NewMemory() *Memory {
	return &Memory{
		Blobs: map[string]*MemoryBlob{},
	}
}

// Prepare implements the Service interface.
func (m *Memory) Prepare(_ context.Context) (Handle, error) {
	// increment id
	m.Next++

	// create handle
	handle := Handle{
		"id": strconv.FormatInt(int64(m.Next), 10),
	}

	return handle, nil
}

// Upload implements the Service interface.
func (m *Memory) Upload(_ context.Context, handle Handle, mediaType string) (Upload, error) {
	// get id
	id, _ := handle["id"].(string)
	if id == "" {
		return nil, ErrInvalidHandle
	}

	// check blob
	_, ok := m.Blobs[id]
	if ok {
		return nil, ErrUsedHandle
	}

	// prepare blob
	blob := &MemoryBlob{
		Type: mediaType,
	}

	// store blob
	m.Blobs[id] = blob

	return &memoryUpload{
		blob: blob,
	}, nil
}

// Download implements the Service interface.
func (m *Memory) Download(_ context.Context, handle Handle) (Download, error) {
	// get id
	id, _ := handle["id"].(string)
	if id == "" {
		return nil, ErrInvalidHandle
	}

	// get blob
	blob, ok := m.Blobs[id]
	if !ok {
		return nil, ErrNotFound
	}

	return &memoryDownload{
		blob: blob,
	}, nil
}

// Delete implements the Service interface.
func (m *Memory) Delete(_ context.Context, handle Handle) (bool, error) {
	// get id
	id, _ := handle["id"].(string)
	if id == "" {
		return false, ErrInvalidHandle
	}

	// check blob
	if _, ok := m.Blobs[id]; !ok {
		return false, ErrNotFound
	}

	// delete blob
	delete(m.Blobs, id)

	return true, nil
}

// Cleanup implements the Service interface.
func (m *Memory) Cleanup(_ context.Context) error {
	return nil
}

type memoryUpload struct {
	blob *MemoryBlob
}

// func (u *memoryUpload) Resume() (int64, error) {
// 	panic("implement me")
// }

func (u *memoryUpload) Write(data []byte) (int, error) {
	// append data
	u.blob.Bytes = append(u.blob.Bytes, data...)

	return len(data), nil
}

// func (u *memoryUpload) Suspend() (int64, error) {
// 	panic("implement me")
// }

func (u *memoryUpload) Abort() error {
	return nil
}

func (u *memoryUpload) Close() error {
	return nil
}

type memoryDownload struct {
	blob *MemoryBlob
	pos  int
}

// func (u *memoryDownload) Seek(offset int64, whence int) (int64, error) {
// 	panic("implement me")
// }

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
