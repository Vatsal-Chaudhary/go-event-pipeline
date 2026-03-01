package archive

import (
	"bytes"
	"context"
	"fmt"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type s3Uploader struct {
	client *s3.Client
}

func NewS3BatchArchiver(region, accessKey, secretKey, bucket string, batchSize int, flushInterval time.Duration, prefixMode string) (*BatchArchiver, error) {
	if region == "" {
		region = "ap-south-1"
	}

	loadOptions := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(region),
	}

	if accessKey != "" && secretKey != "" {
		loadOptions = append(loadOptions, awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), loadOptions...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg)

	return NewBatchArchiver(&s3Uploader{client: client}, bucket, batchSize, flushInterval, prefixMode)
}

func (u *s3Uploader) Upload(ctx context.Context, bucket, objectPath string, payload []byte) error {
	_, err := u.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:          &bucket,
		Key:             &objectPath,
		Body:            bytes.NewReader(payload),
		ContentType:     awsString("application/x-ndjson"),
		ContentEncoding: awsString("gzip"),
	})
	if err != nil {
		return fmt.Errorf("upload to s3: %w", err)
	}

	return nil
}

func awsString(s string) *string {
	return &s
}
