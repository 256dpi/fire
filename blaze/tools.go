package blaze

import "io"

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
