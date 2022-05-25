package blaze

import (
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/assert"
)

func TestMinioService(t *testing.T) {
	client, err := minio.New("0.0.0.0:9000", &minio.Options{
		Creds: credentials.NewStaticV4("minioadmin", "minioadmin", ""),
	})
	assert.NoError(t, err)

	svc := NewMinio(client, "blaze")
	TestService(t, svc)
}

func TestMinioServiceSeek(t *testing.T) {
	client, err := minio.New("0.0.0.0:9000", &minio.Options{
		Creds: credentials.NewStaticV4("minioadmin", "minioadmin", ""),
	})
	assert.NoError(t, err)

	svc := NewMinio(client, "blaze")
	TestServiceSeek(t, svc, false, true)
}
