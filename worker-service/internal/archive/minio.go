package archive

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"path"
	"sync"
	"time"

	"compress/gzip"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type BatchArchiver struct {
	client        *minio.Client
	bucket        string
	batchSize     int
	flushInterval time.Duration
	prefixMode    string

	mu      sync.Mutex
	buffers map[string][][]byte
	closed  bool

	flushCh chan struct{}
	stopCh  chan struct{}

	uploadWG sync.WaitGroup
	loopWG   sync.WaitGroup
}

func NewBatchArchiver(endpoint, accessKey, secretKey, bucket string, batchSize int, flushInterval time.Duration, prefixMode string) (*BatchArchiver, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}
	if batchSize <= 0 {
		batchSize = 100
	}
	if flushInterval <= 0 {
		flushInterval = 5 * time.Second
	}
	if prefixMode == "" {
		prefixMode = "status"
	}

	archiver := &BatchArchiver{
		client:        client,
		bucket:        bucket,
		batchSize:     batchSize,
		flushInterval: flushInterval,
		prefixMode:    prefixMode,
		buffers:       make(map[string][][]byte),
		flushCh:       make(chan struct{}, 1),
		stopCh:        make(chan struct{}),
	}

	archiver.loopWG.Add(1)
	go archiver.flusherLoop()

	return archiver, nil
}

func (b *BatchArchiver) Enqueue(status string, eventData interface{}) error {
	jsonData, err := json.Marshal(eventData)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	if status == "" {
		return fmt.Errorf("status is required")
	}

	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return fmt.Errorf("archiver is closed")
	}
	b.buffers[status] = append(b.buffers[status], jsonData)
	shouldFlush := len(b.buffers[status]) >= b.batchSize
	b.mu.Unlock()

	if shouldFlush {
		select {
		case b.flushCh <- struct{}{}:
		default:
		}
	}

	return nil
}

func (b *BatchArchiver) Flush() {
	b.flushWithContext(context.Background())
}

func (b *BatchArchiver) Close() error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	b.mu.Unlock()

	close(b.stopCh)
	b.loopWG.Wait()
	b.flushWithContext(context.Background())
	b.uploadWG.Wait()

	return nil
}

func (b *BatchArchiver) flusherLoop() {
	defer b.loopWG.Done()

	ticker := time.NewTicker(b.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			b.flushWithContext(context.Background())
		case <-b.flushCh:
			b.flushWithContext(context.Background())
		case <-b.stopCh:
			return
		}
	}
}

func (b *BatchArchiver) flushWithContext(ctx context.Context) {
	batches := b.snapshotBatches()
	for status, records := range batches {
		status := status
		records := records
		b.uploadWG.Add(1)
		go func() {
			defer b.uploadWG.Done()
			if err := b.uploadBatch(ctx, status, records); err != nil {
				log.Printf("archive upload failed for status=%s size=%d: %v", status, len(records), err)
				b.requeue(status, records)
			}
		}()
	}
}

func (b *BatchArchiver) snapshotBatches() map[string][][]byte {
	b.mu.Lock()
	defer b.mu.Unlock()

	batches := make(map[string][][]byte)
	for status, records := range b.buffers {
		if len(records) == 0 {
			continue
		}
		copied := make([][]byte, len(records))
		copy(copied, records)
		batches[status] = copied
		b.buffers[status] = nil
	}

	return batches
}

func (b *BatchArchiver) uploadBatch(ctx context.Context, status string, records [][]byte) error {
	compressed, err := compressNDJSON(records)
	if err != nil {
		return fmt.Errorf("gzip batch: %w", err)
	}

	objectPath, err := b.objectKey(status, time.Now().UTC())
	if err != nil {
		return err
	}

	_, err = b.client.PutObject(ctx, b.bucket, objectPath, bytes.NewReader(compressed), int64(len(compressed)), minio.PutObjectOptions{
		ContentType:     "application/x-ndjson",
		ContentEncoding: "gzip",
	})
	if err != nil {
		return fmt.Errorf("upload to minio: %w", err)
	}

	return nil
}

func (b *BatchArchiver) requeue(status string, records [][]byte) {
	b.mu.Lock()
	defer b.mu.Unlock()

	current := b.buffers[status]
	retried := make([][]byte, 0, len(records)+len(current))
	retried = append(retried, records...)
	retried = append(retried, current...)
	b.buffers[status] = retried
}

func (b *BatchArchiver) objectKey(status string, ts time.Time) (string, error) {
	if b.prefixMode != "status" {
		return "", fmt.Errorf("unsupported ARCHIVE_PREFIX_MODE: %s", b.prefixMode)
	}

	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		return "", fmt.Errorf("random suffix: %w", err)
	}

	name := fmt.Sprintf("batch_%d_%x.ndjson.gz", ts.UnixMilli(), suffix)
	return path.Join(
		status,
		fmt.Sprintf("%04d", ts.Year()),
		fmt.Sprintf("%02d", ts.Month()),
		fmt.Sprintf("%02d", ts.Day()),
		fmt.Sprintf("%02d", ts.Hour()),
		name,
	), nil
}

func compressNDJSON(records [][]byte) ([]byte, error) {
	var ndjson bytes.Buffer
	for _, record := range records {
		if _, err := ndjson.Write(record); err != nil {
			return nil, err
		}
		if err := ndjson.WriteByte('\n'); err != nil {
			return nil, err
		}
	}

	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	if _, err := gz.Write(ndjson.Bytes()); err != nil {
		_ = gz.Close()
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}

	return compressed.Bytes(), nil
}
