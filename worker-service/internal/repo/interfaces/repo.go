package interfaces

import (
	"context"
	"worker-service/internal/models"
)

type EventRepo interface {
	ProcessRawEvent(ctx context.Context, event *models.RawEvent) error
	ProcessRawEventsBatch(ctx context.Context, events []*models.RawEvent) error
}
