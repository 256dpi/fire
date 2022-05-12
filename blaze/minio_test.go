package blaze

import (
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
)

func TestMinioService(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		client, err := minio.New("0.0.0.0:9000", &minio.Options{
			Creds: credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		})
		assert.NoError(t, err)

		svc := NewMinio(client, "blaze")
		abstractServiceTest(t, svc)
	})
}

func TestMinioServiceSeek(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		client, err := minio.New("0.0.0.0:9000", &minio.Options{
			Creds: credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		})
		assert.NoError(t, err)

		svc := NewMinio(client, "blaze")
		abstractServiceSeekTest(t, svc, false, true)
	})
}
