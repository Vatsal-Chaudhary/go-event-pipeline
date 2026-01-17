package models

import (
	"time"
)

type CampaignStats struct {
	CampaignID       string    `db:"campaign_id" json:"campaign_id"`
	TotalImpressions int64     `db:"total_impressions" json:"total_impressions"`
	TotalClicks      int64     `db:"total_clicks" json:"total_clicks"`
	TotalConversions int64     `db:"total_conversions" json:"total_conversions"`
	HourBucket       time.Time `db:"hour_bucket" json:"hour_bucket"`
}
