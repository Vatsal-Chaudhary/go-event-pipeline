package archive

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type minioUploader struct {
	client *minio.Client
}

func NewMinIOBatchArchiver(endpoint, accessKey, secretKey, bucket string, batchSize int, flushInterval time.Duration, prefixMode string) (*BatchArchiver, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}

	return NewBatchArchiver(&minioUploader{client: client}, bucket, batchSize, flushInterval, prefixMode)
}

func (u *minioUploader) Upload(ctx context.Context, bucket, objectPath string, payload []byte) error {
	_, err := u.client.PutObject(ctx, bucket, objectPath, bytes.NewReader(payload), int64(len(payload)), minio.PutObjectOptions{
		ContentType:     "application/x-ndjson",
		ContentEncoding: "gzip",
	})
	if err != nil {
		return fmt.Errorf("upload to minio: %w", err)
	}

	return nil
}
