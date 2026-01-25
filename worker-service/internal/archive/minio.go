package archive

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinIOArchiver struct {
	client *minio.Client
	bucket string
}

func NewMinIOArchiver(endpoint, accessKey, secretKey, bucket string) (*MinIOArchiver, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}

	return &MinIOArchiver{
		client: client,
		bucket: bucket,
	}, nil
}

func (m *MinIOArchiver) Archive(ctx context.Context, eventID string, eventData interface{}, timestamp time.Time) error {
	// Serialize event to JSON
	jsonData, err := json.Marshal(eventData)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	// Generate path: year/month/day/event_id.json
	objectPath := fmt.Sprintf("%d/%02d/%02d/%s.json",
		timestamp.Year(),
		timestamp.Month(),
		timestamp.Day(),
		eventID,
	)

	// Upload to MinIO
	_, err = m.client.PutObject(ctx, m.bucket, objectPath, bytes.NewReader(jsonData), int64(len(jsonData)), minio.PutObjectOptions{
		ContentType: "application/json",
	})
	if err != nil {
		return fmt.Errorf("upload to minio: %w", err)
	}

	return nil
}
