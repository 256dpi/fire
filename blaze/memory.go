package blaze

import (
	"context"
	"io"
	"strconv"
	"sync"

	"github.com/256dpi/xo"
)

var errStreamClosed = xo.BF("stream closed")

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
		return nil, ErrInvalidHandle.Wrap()
	}

	// check blob
	_, ok := m.Blobs[id]
	if ok {
		return nil, ErrUsedHandle.Wrap()
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
		return nil, ErrInvalidHandle.Wrap()
	}

	// get blob
	blob, ok := m.Blobs[id]
	if !ok {
		return nil, ErrNotFound.Wrap()
	}

	return &memoryDownload{
		blob: blob,
	}, nil
}

// Delete implements the Service interface.
func (m *Memory) Delete(_ context.Context, handle Handle) error {
	// get id
	id, _ := handle["id"].(string)
	if id == "" {
		return ErrInvalidHandle.Wrap()
	}

	// delete blob
	delete(m.Blobs, id)

	return nil
}

// Cleanup implements the Service interface.
func (m *Memory) Cleanup(_ context.Context) error {
	return nil
}

type memoryUpload struct {
	blob   *MemoryBlob
	closed bool
	mutex  sync.Mutex
}

// func (u *memoryUpload) Resume() (int64, error) {
// 	panic("implement me")
// }

func (u *memoryUpload) Write(data []byte) (int, error) {
	// acquire mutex
	u.mutex.Lock()
	defer u.mutex.Unlock()

	// check flag
	if u.closed {
		return 0, errStreamClosed.Wrap()
	}

	// append data
	u.blob.Bytes = append(u.blob.Bytes, data...)

	return len(data), nil
}

// func (u *memoryUpload) Suspend() (int64, error) {
// 	panic("implement me")
// }

func (u *memoryUpload) Abort() error {
	// acquire mutex
	u.mutex.Lock()
	defer u.mutex.Unlock()

	// check flag
	if u.closed {
		return errStreamClosed.Wrap()
	}

	return nil
}

func (u *memoryUpload) Close() error {
	// acquire mutex
	u.mutex.Lock()
	defer u.mutex.Unlock()

	// check flag
	if u.closed {
		return errStreamClosed.Wrap()
	}

	return nil
}

type memoryDownload struct {
	blob     *MemoryBlob
	position int64
	closed   bool
	mutex    sync.Mutex
}

func (d *memoryDownload) Seek(offset int64, whence int) (int64, error) {
	// acquire mutex
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// check flag
	if d.closed {
		return 0, errStreamClosed.Wrap()
	}

	// get position
	position := d.position

	// adjust position
	switch whence {
	case io.SeekStart:
		position = offset
	case io.SeekCurrent:
		position = position + offset
	case io.SeekEnd:
		position = int64(len(d.blob.Bytes)) - offset
	}

	// check position
	if position < 0 {
		return 0, ErrInvalidPosition.Wrap()
	}

	// update position
	d.position = position

	return d.position, nil
}

func (d *memoryDownload) Read(buf []byte) (int, error) {
	// acquire mutex
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// check flag
	if d.closed {
		return 0, errStreamClosed.Wrap()
	}

	// check EOF
	if d.position >= int64(len(d.blob.Bytes)) {
		return 0, io.EOF
	}

	// copy bytes
	n := copy(buf, d.blob.Bytes[d.position:])
	d.position += int64(n)

	return n, nil
}

func (d *memoryDownload) Close() error {
	// acquire mutex
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// check flag
	if d.closed {
		return errStreamClosed.Wrap()
	}

	// set flag
	d.closed = true

	return nil
}
