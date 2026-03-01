package utils

import (
	"testing"

	"worker-service/internal/models"
)

func TestValidateKafkaMessage(t *testing.T) {
	valid := &models.KafkaEventMessage{
		EventType:  models.EventTypeClick,
		UserID:     "user-1",
		CampaignID: "cmp-1",
		GeoCountry: "IN",
		DeviceType: "mobile",
	}

	tests := []struct {
		name    string
		msg     *models.KafkaEventMessage
		wantErr bool
		field   string
	}{
		{name: "nil message", msg: nil, wantErr: true, field: "message"},
		{name: "invalid event type", msg: &models.KafkaEventMessage{EventType: "bad", UserID: "u", CampaignID: "c", GeoCountry: "IN", DeviceType: "mobile"}, wantErr: true, field: "event_type"},
		{name: "missing user id", msg: &models.KafkaEventMessage{EventType: models.EventTypeClick, CampaignID: "c", GeoCountry: "IN", DeviceType: "mobile"}, wantErr: true, field: "user_id"},
		{name: "missing campaign id", msg: &models.KafkaEventMessage{EventType: models.EventTypeClick, UserID: "u", GeoCountry: "IN", DeviceType: "mobile"}, wantErr: true, field: "campaign_id"},
		{name: "missing geo country", msg: &models.KafkaEventMessage{EventType: models.EventTypeClick, UserID: "u", CampaignID: "c", DeviceType: "mobile"}, wantErr: true, field: "geo_country"},
		{name: "invalid geo country length", msg: &models.KafkaEventMessage{EventType: models.EventTypeClick, UserID: "u", CampaignID: "c", GeoCountry: "IND", DeviceType: "mobile"}, wantErr: true, field: "geo_country"},
		{name: "missing device type", msg: &models.KafkaEventMessage{EventType: models.EventTypeClick, UserID: "u", CampaignID: "c", GeoCountry: "IN"}, wantErr: true, field: "device_type"},
		{name: "valid message", msg: valid, wantErr: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateKafkaMessage(tc.msg)
			if tc.wantErr {
				if err == nil {
					t.Fatal("ValidateKafkaMessage() error = nil, want non-nil")
				}
				validationErr, ok := err.(*ValidationError)
				if !ok {
					t.Fatalf("ValidateKafkaMessage() error type = %T, want *ValidationError", err)
				}
				if validationErr.Field != tc.field {
					t.Fatalf("ValidationError.Field = %q, want %q", validationErr.Field, tc.field)
				}
				return
			}

			if err != nil {
				t.Fatalf("ValidateKafkaMessage() error = %v, want nil", err)
			}
		})
	}
}
