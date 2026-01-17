package utils

import (
	"fmt"
	"worker-service/internal/models"
)

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

func ValidateKafkaMessage(msg *models.KafkaEventMessage) error {
	if msg == nil {
		return &ValidationError{Field: "message", Message: "message is nil"}
	}

	if msg.EventType != models.EventTypeImpression &&
		msg.EventType != models.EventTypeClick &&
		msg.EventType != models.EventTypeConversion {
		return &ValidationError{Field: "event_type", Message: fmt.Sprintf("invalid event_type: %s", msg.EventType)}
	}

	if msg.UserID == "" {
		return &ValidationError{Field: "user_id", Message: "user_id is required"}
	}

	if msg.CampaignID == "" {
		return &ValidationError{Field: "campaign_id", Message: "campaign_id is required"}
	}

	if msg.GeoCountry == "" {
		return &ValidationError{Field: "geo_country", Message: "geo_country is required"}
	}

	if len(msg.GeoCountry) != 2 {
		return &ValidationError{Field: "geo_country", Message: "must be 2-letter ISO code"}
	}

	if msg.DeviceType == "" {
		return &ValidationError{Field: "device_type", Message: "device_type is required"}
	}

	return nil
}
