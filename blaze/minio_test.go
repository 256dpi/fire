package blaze

import (
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func TestMinioService(t *testing.T) {
	TestService(t, NewMinio(makeMinioClient(), "blaze"))
}

func TestMinioServiceSeek(t *testing.T) {
	TestServiceSeek(t, NewMinio(makeMinioClient(), "blaze"))
}

func makeMinioClient() *minio.Client {
	client, err := minio.New("0.0.0.0:9000", &minio.Options{
		Creds: credentials.NewStaticV4("minioadmin", "minioadmin", ""),
	})
	if err != nil {
		panic(err)
	}

	return client
}
