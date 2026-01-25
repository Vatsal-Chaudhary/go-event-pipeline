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

	"github.com/google/uuid"
)

type EventService struct {
	repo       interfaces.EventRepo
	archiver   *archive.MinIOArchiver
	dedup      *dedupe.RedisDeduplicator
	fraudCheck *fraud.LambdaClient

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
	Archiver   *archive.MinIOArchiver
	Dedup      *dedupe.RedisDeduplicator
	FraudCheck *fraud.LambdaClient
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
// 3. Check fraud with Lambda
// 4. Persist to DB (if not fraud)
func (s *EventService) ProcessEvent(ctx context.Context, msg *models.KafkaEventMessage) error {
	start := time.Now()

	// Validate message
	if err := utils.ValidateKafkaMessage(msg); err != nil {
		s.metrics.incrementFailed()
		return fmt.Errorf("validation: %w", err)
	}

	event := s.transformMessage(msg)

	// Generate event ID if not present
	if event.EventID == "" {
		event.EventID = uuid.New().String()
	}

	// Step 1: Archive to MinIO
	if err := s.archiver.Archive(ctx, event.EventID, event, event.CreatedAt); err != nil {
		log.Printf("Archive failed for event %s: %v", event.EventID, err)
		s.metrics.incrementFailed()
		return fmt.Errorf("archive: %w", err)
	}
	s.metrics.incrementArchived()
	log.Printf("✓ Archived event %s to MinIO", event.EventID)

	// Step 2: Deduplicate with Redis
	isDuplicate, err := s.dedup.IsDuplicate(ctx, event.EventID)
	if err != nil {
		log.Printf("Deduplication check failed for event %s: %v", event.EventID, err)
		s.metrics.incrementFailed()
		return fmt.Errorf("dedupe: %w", err)
	}
	if isDuplicate {
		log.Printf("⚠ Duplicate detected: %s (skipping)", event.EventID)
		s.metrics.incrementDuplicates()
		return nil // Stop processing, but don't treat as error
	}
	log.Printf("✓ Event %s is unique", event.EventID)

	// Step 3: Check fraud with Lambda
	riskScore, err := s.fraudCheck.CheckFraud(ctx, event.IPAddress, event.UserID)
	if err != nil {
		log.Printf("Fraud check failed for event %s: %v", event.EventID, err)
		s.metrics.incrementFailed()
		return fmt.Errorf("fraud check: %w", err)
	}
	event.RiskScore = riskScore
	log.Printf("✓ Fraud check complete for event %s: risk_score=%d", event.EventID, riskScore)

	// If high risk, log and skip DB save
	if riskScore > 80 {
		log.Printf("🚨 FRAUD DETECTED for event %s: risk_score=%d, ip=%s, user=%s",
			event.EventID, riskScore, event.IPAddress, event.UserID)
		s.metrics.incrementFraudDetected()
		return nil // Stop processing, commit offset
	}

	// Step 4: Persist to DB
	if err := s.repo.ProcessRawEvent(ctx, event); err != nil {
		log.Printf("DB persist failed for event %s: %v", event.EventID, err)
		s.metrics.incrementFailed()
		return fmt.Errorf("persist: %w", err)
	}
	log.Printf("✓ Event %s persisted to DB", event.EventID)

	s.metrics.addProcessed(1, time.Since(start))
	return nil
}

// ProcessEventBatch processes multiple events
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
