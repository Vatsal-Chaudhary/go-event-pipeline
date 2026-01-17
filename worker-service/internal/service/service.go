package service

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
	"worker-service/internal/models"
	"worker-service/internal/repo/interfaces"
	"worker-service/internal/utils"
)

type EventService struct {
	repo interfaces.EventRepo

	batchSize     int
	flushInterval time.Duration

	buffer     []*models.RawEvent
	bufferLock sync.Mutex

	metrics    *Metrics
	shutdownCh chan struct{}
	wg         sync.WaitGroup
}

type Metrics struct {
	mu                sync.RWMutex
	TotalProcessed    int64
	TotalFailed       int64
	TotalBatches      int64
	LastProcessedAt   time.Time
	AvgProcessingTime time.Duration
}

type MetricsData struct {
	TotalProcessed    int64
	TotalFailed       int64
	TotalBatches      int64
	LastProcessedAt   time.Time
	AvgProcessingTime time.Duration
}

type Config struct {
	BatchSize     int
	FlushInterval time.Duration
}

func NewEventService(repo interfaces.EventRepo, config Config) *EventService {
	if config.BatchSize == 0 {
		config.BatchSize = 100
	}
	if config.FlushInterval == 0 {
		config.FlushInterval = 5 * time.Second
	}

	service := &EventService{
		repo:          repo,
		batchSize:     config.BatchSize,
		flushInterval: config.FlushInterval,
		buffer:        make([]*models.RawEvent, 0, config.BatchSize),
		metrics:       &Metrics{},
		shutdownCh:    make(chan struct{}),
	}
	service.wg.Add(1)
	go service.periodicFlush()

	return service
}

func (s *EventService) ProcessEvent(ctx context.Context, msg *models.KafkaEventMessage) error {
	if err := utils.ValidateKafkaMessage(msg); err != nil {
		s.metrics.incrementFailed()
		return fmt.Errorf("validation: %w", err)
	}

	event := s.transformMessage(msg)
	shouldFlush := s.addToBuffer(event)

	if shouldFlush {
		return s.flushBuffer(ctx)
	}

	return nil
}

func (s *EventService) ProcessEventBatch(ctx context.Context, messages []*models.KafkaEventMessage) error {
	if len(messages) == 0 {
		return nil
	}

	events := make([]*models.RawEvent, 0, len(messages))
	failCount := 0

	for _, msg := range messages {
		if err := utils.ValidateKafkaMessage(msg); err != nil {
			log.Printf("validation failed: %v", err)
			failCount++
			continue
		}
		events = append(events, s.transformMessage(msg))
	}

	if failCount > 0 {
		s.metrics.addFailed(int64(failCount))
		log.Printf("Failed validation: %d/%d messages", failCount, len(messages))
	}

	if len(events) == 0 {
		return fmt.Errorf("all messages failed validation")
	}

	start := time.Now()
	if err := s.repo.ProcessRawEventsBatch(ctx, events); err != nil {
		s.metrics.addFailed(int64(len(events)))
		return fmt.Errorf("process batch: %w", err)
	}

	s.metrics.addProcessed(int64(len(events)), time.Since(start))
	s.metrics.incrementBatches()

	return nil
}

func (s *EventService) ProcessWithRetry(ctx context.Context, msg *models.KafkaEventMessage, maxRetries int) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt) * time.Second
			time.Sleep(backoff)
		}

		if err := s.ProcessEvent(ctx, msg); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}

	return fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

func (s *EventService) addToBuffer(event *models.RawEvent) bool {
	s.bufferLock.Lock()
	defer s.bufferLock.Unlock()

	s.buffer = append(s.buffer, event)
	return len(s.buffer) >= s.batchSize
}

func (s *EventService) flushBuffer(ctx context.Context) error {
	s.bufferLock.Lock()
	if len(s.buffer) == 0 {
		s.bufferLock.Unlock()
		return nil
	}

	events := make([]*models.RawEvent, len(s.buffer))
	copy(events, s.buffer)
	s.buffer = s.buffer[:0]
	s.bufferLock.Unlock()

	start := time.Now()
	if err := s.repo.ProcessRawEventsBatch(ctx, events); err != nil {
		s.metrics.addFailed(int64(len(events)))
		return err
	}

	s.metrics.addProcessed(int64(len(events)), time.Since(start))
	s.metrics.incrementBatches()

	return nil
}

func (s *EventService) periodicFlush() {
	defer s.wg.Done()
	ticker := time.NewTicker(s.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			s.flushBuffer(ctx)
			cancel()
		case <-s.shutdownCh:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			s.flushBuffer(ctx)
			cancel()
			return
		}
	}
}

func (s *EventService) transformMessage(msg *models.KafkaEventMessage) *models.RawEvent {
	createdAt := time.Now()
	if msg.Timestamp != nil {
		createdAt = *msg.Timestamp
	}

	return &models.RawEvent{
		EventType:  msg.EventType,
		UserID:     msg.UserID,
		CampaignID: msg.CampaignID,
		GeoCountry: msg.GeoCountry,
		DeviceType: msg.DeviceType,
		CreatedAt:  createdAt,
		MetaData:   msg.MetaData,
	}
}

func (s *EventService) GetMetrics() MetricsData {
	s.metrics.mu.RLock()
	defer s.metrics.mu.RUnlock()
	return MetricsData{
		TotalProcessed:    s.metrics.TotalProcessed,
		TotalFailed:       s.metrics.TotalFailed,
		TotalBatches:      s.metrics.TotalBatches,
		LastProcessedAt:   s.metrics.LastProcessedAt,
		AvgProcessingTime: s.metrics.AvgProcessingTime,
	}
}

func (s *EventService) Shutdown(ctx context.Context) error {
	close(s.shutdownCh)

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Metrics) addProcessed(count int64, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalProcessed += count
	m.LastProcessedAt = time.Now()
	m.AvgProcessingTime = duration / time.Duration(count)
}

func (m *Metrics) addFailed(count int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalFailed += count
}

func (m *Metrics) incrementFailed() {
	m.addFailed(1)
}

func (m *Metrics) incrementBatches() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalBatches++
}
