package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"worker-service/internal/models"
	"worker-service/internal/repo/interfaces"
	"worker-service/internal/utils"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

type EventRepo struct {
	db *sqlx.DB
}

func NewEventRepo(db *sqlx.DB) interfaces.EventRepo {
	return &EventRepo{
		db: db,
	}
}

func (r *EventRepo) ProcessRawEvent(ctx context.Context, event *models.RawEvent) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	var metaDataJSON string

	if event.MetaData != nil {
		metaDataBytes, err := json.Marshal(event.MetaData)
		if err != nil {
			return fmt.Errorf("marshal meta_data: %w", err)
		}
		metaDataJSON = string(metaDataBytes)
	} else {
		metaDataJSON = "null"
	}

	insertQuery := `
	INSERT INTO raw_events (
		event_type, user_id, campaign_id, geo_country, device_type,
		created_at, ip_address, risk_score, meta_data
	)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err = tx.ExecContext(ctx, insertQuery,
		event.EventType,
		event.UserID,
		event.CampaignID,
		event.GeoCountry,
		event.DeviceType,
		event.CreatedAt,
		event.IPAddress,
		event.RiskScore,
		metaDataJSON,
	)
	if err != nil {
		return fmt.Errorf("insert a new event: %w", err)
	}

	hourBucket := utils.GetHourBucket(event.CreatedAt)

	impressions, clicks, conversions := 0, 0, 0
	switch event.EventType {
	case models.EventTypeImpression:
		impressions = 1
	case models.EventTypeClick:
		clicks = 1
	case models.EventTypeConversion:
		conversions = 1
	}

	statsQuery := `
	INSERT INTO campaign_stats (
		campaign_id, hour_bucket, total_impressions, total_clicks, total_conversions
	)
	VALUES ($1, $2, $3, $4, $5)
	ON CONFLICT (campaign_id, hour_bucket)
	DO UPDATE SET
		total_impressions = campaign_stats.total_impressions + EXCLUDED.total_impressions,
		total_clicks = campaign_stats.total_clicks + EXCLUDED.total_clicks,
		total_conversions = campaign_stats.total_conversions + EXCLUDED.total_conversions
	`

	_, err = r.db.ExecContext(ctx, statsQuery,
		event.CampaignID,
		hourBucket,
		impressions,
		clicks,
		conversions,
	)
	if err != nil {
		return fmt.Errorf("update error status: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (r *EventRepo) ProcessRawEventsBatch(ctx context.Context, events []*models.RawEvent) error {
	if len(events) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, pq.CopyIn(
		"raw_events",
		"event_type", "user_id", "campaign_id", "geo_country",
		"device_type", "created_at", "ip_address", "risk_score", "meta_data",
	))
	if err != nil {
		return fmt.Errorf("prepare copy: %w", err)
	}

	for _, event := range events {
		var metaDataJSON string
		if event.MetaData != nil {
			metaDataBytes, err := json.Marshal(event.MetaData)
			if err != nil {
				return fmt.Errorf("marshal meta_data: %w", err)
			}
			metaDataJSON = string(metaDataBytes)
		} else {
			metaDataJSON = "null"
		}

		_, err = stmt.ExecContext(ctx,
			event.EventType,
			event.UserID,
			event.CampaignID,
			event.GeoCountry,
			event.DeviceType,
			event.CreatedAt,
			event.IPAddress,
			event.RiskScore,
			metaDataJSON,
		)
		if err != nil {
			return fmt.Errorf("exec copy: %w", err)
		}
	}
	if _, err := stmt.ExecContext(ctx); err != nil {
		return fmt.Errorf("finalize copy: %w", err)
	}
	stmt.Close()

	statsMap := make(map[string]*models.CampaignStats)

	for _, event := range events {
		hourBucket := utils.GetHourBucket(event.CreatedAt)
		key := fmt.Sprintf("%s|%s", event.CampaignID, hourBucket.Format(time.RFC3339))

		if _, exists := statsMap[key]; !exists {
			statsMap[key] = &models.CampaignStats{
				CampaignID: event.CampaignID,
				HourBucket: hourBucket,
			}
		}

		stats := statsMap[key]
		switch event.EventType {
		case models.EventTypeClick:
			stats.TotalClicks++
		case models.EventTypeConversion:
			stats.TotalConversions++
		case models.EventTypeImpression:
			stats.TotalImpressions++
		}
	}

	statsQuery := `
	INSERT INTO campaign_stats (
		campaign_id, hour_bucket, total_impressions, total_clicks, total_conversions
	)
	VALUES ($1, $2, $3, $4, $5)
	ON CONFLICT (campaign_id, hour_bucket)
	DO UPDATE SET
		total_impressions = campaign_stats.total_impressions + EXCLUDED.total_impressions,
		total_clicks = campaign_stats.total_clicks + EXCLUDED.total_clicks,
		total_conversions = campaign_stats.total_conversions + EXCLUDED.total_conversions
	`

	for _, stats := range statsMap {
		_, err := tx.ExecContext(ctx, statsQuery,
			stats.CampaignID,
			stats.HourBucket,
			stats.TotalImpressions,
			stats.TotalClicks,
			stats.TotalConversions,
		)
		if err != nil {
			return fmt.Errorf("upsert stats: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
