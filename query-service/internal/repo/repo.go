package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"

	"query-service/internal/models"
	"query-service/internal/repo/interfaces"
)

type CampaignRepo struct {
	db *sqlx.DB
}

func NewCampaignRepo(db *sqlx.DB) interfaces.CampaignRepo {
	return &CampaignRepo{db: db}
}

func (r *CampaignRepo) GetAllCampaigns(ctx context.Context) ([]models.CampaignStats, error) {
	query := `
		SELECT campaign_id, total_impressions, total_clicks, total_conversions, hour_bucket
		FROM campaign_stats
		ORDER BY hour_bucket DESC
	`

	var campaigns []models.CampaignStats
	if err := r.db.SelectContext(ctx, &campaigns, query); err != nil {
		return nil, fmt.Errorf("select campaigns: %w", err)
	}

	return campaigns, nil
}

func (r *CampaignRepo) GetCampaignByID(ctx context.Context, campaignID string) (*models.CampaignStats, error) {
	query := `
		SELECT campaign_id, total_impressions, total_clicks, total_conversions, hour_bucket
		FROM campaign_stats
		WHERE campaign_id = $1
	`

	var campaign models.CampaignStats
	if err := r.db.GetContext(ctx, &campaign, query, campaignID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get campaign by id: %w", err)
	}

	return &campaign, nil
}
