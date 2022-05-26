package blaze

import (
	"errors"
	"io"
	"sync"
)

// UploadFrom will upload a blob from the provided reader.
func UploadFrom(upload Upload, reader io.Reader) (int64, error) {
	// copy all data
	n, err := io.Copy(upload, reader)
	if err != nil {
		_ = upload.Abort()
		return 0, err
	}

	// close upload
	err = upload.Close()
	if err != nil {
		return 0, err
	}

	return n, nil
}

// DownloadTo will download a blob to the provided writer.
func DownloadTo(download Download, writer io.Writer) error {
	// copy all data
	_, err := io.Copy(writer, download)
	if err != nil {
		_ = download.Close()
		return err
	}

	// close download
	err = download.Close()
	if err != nil {
		return err
	}

	return nil
}

var errPipeUploadAbort = errors.New("pipe upload abort")

type pipeUpload struct {
	pipe *io.PipeWriter
	done chan error
}

// PipeUpload returns an upload that pipes data to the reader yielded to the
// provided callback. This function is useful to upload data to a service that
// expects a reader. Errors from the callback are returned by the upload either
// on write or on close.
func PipeUpload(fn func(upload io.Reader) error) Upload {
	// prepare pipe
	r, w := io.Pipe()

	// prepare upload
	upload := &pipeUpload{
		pipe: w,
		done: make(chan error, 1),
	}

	// start upload
	go func() {
		// yield reader
		err := fn(r)

		// handle abort
		if errors.Is(err, errPipeUploadAbort) {
			close(upload.done)
			return
		}

		// close reader with error
		if err != nil {
			_ = r.CloseWithError(err)
		} else {
			_ = r.Close()
		}

		// return error
		upload.done <- err

		// close upload
		close(upload.done)
	}()

	return upload
}

func (p *pipeUpload) Write(data []byte) (int, error) {
	// write data
	n, err := p.pipe.Write(data)
	if err == nil {
		return n, nil
	}

	// drain error
	select {
	case <-p.done:
	default:
	}

	return n, err
}

func (p *pipeUpload) Abort() error {
	// abort upload
	err := p.pipe.CloseWithError(errPipeUploadAbort)
	if err != nil {
		return err
	}

	// await return
	err = <-p.done
	if err != nil && !errors.Is(err, errPipeUploadAbort) {
		return err
	}

	return nil
}

func (p *pipeUpload) Close() error {
	// close upload
	err := p.pipe.Close()
	if err != nil {
		return err
	}

	// await return
	err = <-p.done
	if err != nil {
		return err
	}

	return nil
}

type seekableDownload struct {
	size   int64
	source func(offset int64) (io.ReadCloser, error)
	offset int64
	stream io.ReadCloser
	mutex  sync.Mutex
}

// SeekableDownload returns a download that is seekable from a function that
// opens a download stream at a given offset. This function is useful to
// efficiently implemented a seekable download over a service that does not
// provide native seekable downloads.
func SeekableDownload(size int64, source func(offset int64) (io.ReadCloser, error)) Download {
	return &seekableDownload{
		size:   size,
		source: source,
	}
}

func (s *seekableDownload) Seek(offset int64, whence int) (int64, error) {
	// acquire mutex
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// compute new offset
	var newOffset int64
	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = s.offset + offset
	case io.SeekEnd:
		newOffset = s.size + offset
	}

	// handle underflow
	if newOffset < 0 {
		return 0, ErrInvalidPosition.Wrap()
	}

	// return if not changed
	if newOffset == s.offset {
		return s.offset, nil
	}

	// set offset
	s.offset = newOffset

	// close stream if open
	if s.stream != nil {
		err := s.stream.Close()
		if err != nil {
			return 0, err
		}
		s.stream = nil
	}

	return s.offset, nil
}

func (s *seekableDownload) Read(buf []byte) (int, error) {
	// acquire mutex
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// check offset
	if s.offset >= s.size {
		return 0, io.EOF
	}

	// ensure stream
	if s.stream == nil {
		var err error
		s.stream, err = s.source(s.offset)
		if err != nil {
			return 0, err
		}
	}

	// read from stream
	n, err := s.stream.Read(buf)
	if n > 0 && err == io.EOF {
		err = nil
	}

	// adjust offset
	s.offset += int64(n)

	return n, err
}

func (s *seekableDownload) Close() error {
	// acquire mutex
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// return if no stream
	if s.stream == nil {
		return nil
	}

	// close stream
	err := s.stream.Close()
	if err != nil {
		return err
	}

	// clear stream
	s.stream = nil

	return nil
}
