package blaze

import "testing"

func TestMinioService(t *testing.T) {
	TestService(t, makeMinioClient())
}

func TestMinioServiceSeek(t *testing.T) {
	TestServiceSeek(t, makeMinioClient())
}

func makeMinioClient() *Minio {
	client, err := NewMinioURL("http://minioadmin:minioadmin@localhost:9000/blaze")
	if err != nil {
		panic(err)
	}

	return client
}
