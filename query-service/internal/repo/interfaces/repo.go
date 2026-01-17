package interfaces

import (
	"context"

	"query-service/internal/models"
)

type CampaignRepo interface {
	GetAllCampaigns(ctx context.Context) ([]models.CampaignStats, error)
	GetCampaignByID(ctx context.Context, campaignID string) (*models.CampaignStats, error)
}
