package blaze

import "testing"

// Note: We skip tests in short mode because they require a running Minio server.

func TestMinioService(t *testing.T) {
	if !testing.Short() {
		TestService(t, makeMinioClient())
	}
}

func TestMinioServiceSeek(t *testing.T) {
	if !testing.Short() {
		TestServiceSeek(t, makeMinioClient())
	}
}

func makeMinioClient() *Minio {
	client, err := NewMinioURL("http://minioadmin:minioadmin@127.0.0.1:9000/blaze")
	if err != nil {
		panic(err)
	}

	return client
}
