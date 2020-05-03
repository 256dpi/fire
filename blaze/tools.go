package blaze

import (
	"context"
	"io"
)

// UploadFrom will upload a blob from the provided reader.
func UploadFrom(ctx context.Context, svc Service, handle Handle, contentType string, reader io.Reader) (int64, error) {
	// start upload
	upload, err := svc.Upload(ctx, handle, contentType)
	if err != nil {
		return 0, err
	}

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
func DownloadTo(ctx context.Context, svc Service, handle Handle, writer io.Writer) error {
	// start download
	download, err := svc.Download(ctx, handle)
	if err != nil {
		return err
	}

	// copy all data
	_, err = io.Copy(writer, download)
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
