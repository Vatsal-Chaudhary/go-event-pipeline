package models

import "time"

type EventType string

const (
	EventTypeImpression EventType = "impression"
	EventTypeClick      EventType = "click"
	EventTypeConversion EventType = "conversion"
)

type RawEvent struct {
	EventID    string                 `json:"event_id,omitempty"`
	EventType  EventType              `json:"event_type"`
	UserID     string                 `json:"user_id"`
	CampaignID string                 `json:"campaign_id"`
	GeoCountry string                 `json:"geo_country"`
	DeviceType string                 `json:"device_type"`
	CreatedAt  time.Time              `json:"created_at"`
	IPAddress  string                 `json:"ip_address,omitempty"`
	MetaData   map[string]interface{} `json:"meta_data,omitempty"`
}
type KafkaEventMessage struct {
	EventID    string                 `json:"event_id,omitempty"`
	EventType  EventType              `json:"event_type"`
	UserID     string                 `json:"user_id"`
	CampaignID string                 `json:"campaign_id"`
	GeoCountry string                 `json:"geo_country"`
	DeviceType string                 `json:"device_type"`
	Timestamp  *time.Time             `json:"timestamp,omitempty"`
	IPAddress  string                 `json:"ip_address,omitempty"`
	MetaData   map[string]interface{} `json:"meta_data,omitempty"`
}
