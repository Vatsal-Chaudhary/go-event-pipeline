package service

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"worker-service/internal/archive"
	"worker-service/internal/dedupe"
	"worker-service/internal/fraud"
	"worker-service/internal/models"
	"worker-service/internal/repo/interfaces"
	"worker-service/internal/utils"
)

type EventService struct {
	repo       interfaces.EventRepo
	archiver   *archive.BatchArchiver
	dedup      *dedupe.RedisDeduplicator
	fraudCheck fraud.Scorer

	metrics    *Metrics
	shutdownCh chan struct{}
	wg         sync.WaitGroup
}

type Metrics struct {
	mu                 sync.RWMutex
	TotalProcessed     int64
	TotalFailed        int64
	TotalDuplicates    int64
	TotalFraudDetected int64
	TotalArchived      int64
	LastProcessedAt    time.Time
	AvgProcessingTime  time.Duration
}

type MetricsData struct {
	TotalProcessed     int64
	TotalFailed        int64
	TotalDuplicates    int64
	TotalFraudDetected int64
	TotalArchived      int64
	LastProcessedAt    time.Time
	AvgProcessingTime  time.Duration
}

type Config struct {
	Archiver   *archive.BatchArchiver
	Dedup      *dedupe.RedisDeduplicator
	FraudCheck fraud.Scorer
}

func NewEventService(repo interfaces.EventRepo, config Config) *EventService {
	service := &EventService{
		repo:       repo,
		archiver:   config.Archiver,
		dedup:      config.Dedup,
		fraudCheck: config.FraudCheck,
		metrics:    &Metrics{},
		shutdownCh: make(chan struct{}),
	}

	return service
}

// ProcessEvent processes a single event through the Smart Worker chain:
// 1. Archive to MinIO
// 2. Deduplicate with Redis
// 3. Check fraud score
// 4. Persist to DB (if not fraud)
func (s *EventService) ProcessEvent(ctx context.Context, msg *models.KafkaEventMessage) error {
	start := time.Now()

	// Validate message
	if err := utils.ValidateKafkaMessage(msg); err != nil {
		s.metrics.incrementFailed()
		return fmt.Errorf("validation: %w", err)
	}

	event := s.transformMessage(msg)

	if err := s.archiver.Enqueue("raw", event); err != nil {
		log.Printf("Archive enqueue failed for status=raw event %s: %v", event.EventID, err)
		s.metrics.incrementFailed()
		return fmt.Errorf("archive raw: %w", err)
	}
	s.metrics.incrementArchived()
	log.Printf("✓ Enqueued event %s for archive status=raw", event.EventID)

	isDuplicate, err := s.dedup.IsDuplicate(ctx, event.EventID)
	if err != nil {
		log.Printf("Deduplication check failed for event %s: %v", event.EventID, err)
		s.metrics.incrementFailed()
		return fmt.Errorf("dedupe: %w", err)
	}
	if isDuplicate {
		if err := s.archiver.Enqueue("duplicate", event); err != nil {
			log.Printf("Archive enqueue failed for status=duplicate event %s: %v", event.EventID, err)
			s.metrics.incrementFailed()
			return fmt.Errorf("archive duplicate: %w", err)
		}
		s.metrics.incrementArchived()
		log.Printf("⚠ Duplicate detected: %s (skipping)", event.EventID)
		s.metrics.incrementDuplicates()
		return nil // Stop processing, but don't treat as error
	}
	log.Printf("✓ Event %s is unique", event.EventID)

	riskScore, err := s.fraudCheck.CheckFraud(ctx, event.IPAddress, event.UserID)
	if err != nil {
		log.Printf("Fraud check failed for event %s: %v", event.EventID, err)
		s.metrics.incrementFailed()
		return fmt.Errorf("fraud check: %w", err)
	}
	event.RiskScore = riskScore
	log.Printf("✓ Fraud check complete for event %s: risk_score=%d", event.EventID, riskScore)

	if riskScore > 80 {
		if err := s.archiver.Enqueue("fraud", event); err != nil {
			log.Printf("Archive enqueue failed for status=fraud event %s: %v", event.EventID, err)
			s.metrics.incrementFailed()
			return fmt.Errorf("archive fraud: %w", err)
		}
		s.metrics.incrementArchived()
		log.Printf("🚨 FRAUD DETECTED for event %s: risk_score=%d, ip=%s, user=%s",
			event.EventID, riskScore, event.IPAddress, event.UserID)
		s.metrics.incrementFraudDetected()
		return nil // Stop processing, commit offset
	}

	if err := s.archiver.Enqueue("accepted", event); err != nil {
		log.Printf("Archive enqueue failed for status=accepted event %s: %v", event.EventID, err)
		s.metrics.incrementFailed()
		return fmt.Errorf("archive accepted: %w", err)
	}
	s.metrics.incrementArchived()

	if err := s.repo.ProcessRawEvent(ctx, event); err != nil {
		log.Printf("DB persist failed for event %s: %v", event.EventID, err)
		s.metrics.incrementFailed()
		return fmt.Errorf("persist: %w", err)
	}
	log.Printf("✓ Event %s persisted to DB", event.EventID)

	s.metrics.addProcessed(1, time.Since(start))
	return nil
}

func (s *EventService) ProcessEventBatch(ctx context.Context, messages []*models.KafkaEventMessage) error {
	if len(messages) == 0 {
		return nil
	}

	log.Printf("Processing batch of %d events...", len(messages))

	for _, msg := range messages {
		if err := s.ProcessEvent(ctx, msg); err != nil {
			// Log error but continue processing other events
			log.Printf("Error processing event: %v", err)
		}
	}

	return nil
}

func (s *EventService) transformMessage(msg *models.KafkaEventMessage) *models.RawEvent {
	createdAt := time.Now()
	if msg.Timestamp != nil {
		createdAt = *msg.Timestamp
	}

	return &models.RawEvent{
		EventID:    msg.EventID,
		EventType:  msg.EventType,
		UserID:     msg.UserID,
		CampaignID: msg.CampaignID,
		GeoCountry: msg.GeoCountry,
		DeviceType: msg.DeviceType,
		CreatedAt:  createdAt,
		IPAddress:  msg.IPAddress,
		MetaData:   msg.MetaData,
	}
}

func (s *EventService) GetMetrics() MetricsData {
	s.metrics.mu.RLock()
	defer s.metrics.mu.RUnlock()
	return MetricsData{
		TotalProcessed:     s.metrics.TotalProcessed,
		TotalFailed:        s.metrics.TotalFailed,
		TotalDuplicates:    s.metrics.TotalDuplicates,
		TotalFraudDetected: s.metrics.TotalFraudDetected,
		TotalArchived:      s.metrics.TotalArchived,
		LastProcessedAt:    s.metrics.LastProcessedAt,
		AvgProcessingTime:  s.metrics.AvgProcessingTime,
	}
}

func (s *EventService) Shutdown(ctx context.Context) error {
	close(s.shutdownCh)
	if err := s.archiver.Close(); err != nil {
		return fmt.Errorf("archiver close: %w", err)
	}

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

func (m *Metrics) incrementFailed() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalFailed++
}

func (m *Metrics) incrementDuplicates() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalDuplicates++
}

func (m *Metrics) incrementFraudDetected() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalFraudDetected++
}

func (m *Metrics) incrementArchived() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalArchived++
}
