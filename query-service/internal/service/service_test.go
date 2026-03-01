package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"query-service/internal/models"
)

type mockCampaignRepo struct {
	getAllFunc  func(ctx context.Context) ([]models.CampaignStats, error)
	getByIDFunc func(ctx context.Context, campaignID string) (*models.CampaignStats, error)
}

func (m *mockCampaignRepo) GetAllCampaigns(ctx context.Context) ([]models.CampaignStats, error) {
	return m.getAllFunc(ctx)
}

func (m *mockCampaignRepo) GetCampaignByID(ctx context.Context, campaignID string) (*models.CampaignStats, error) {
	return m.getByIDFunc(ctx, campaignID)
}

func TestCampaignService_GetAllCampaignsSuccess(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Hour)
	expected := []models.CampaignStats{{CampaignID: "cmp-1", HourBucket: now}}

	svc := NewCampaignService(&mockCampaignRepo{
		getAllFunc: func(ctx context.Context) ([]models.CampaignStats, error) {
			return expected, nil
		},
		getByIDFunc: func(ctx context.Context, campaignID string) (*models.CampaignStats, error) {
			return nil, nil
		},
	})

	got, err := svc.GetAllCampaigns(context.Background())
	if err != nil {
		t.Fatalf("GetAllCampaigns() error = %v", err)
	}

	if len(got) != 1 || got[0].CampaignID != "cmp-1" || !got[0].HourBucket.Equal(now) {
		t.Fatalf("GetAllCampaigns() got = %#v, want %#v", got, expected)
	}
}

func TestCampaignService_GetAllCampaignsWrapsRepoError(t *testing.T) {
	repoErr := errors.New("db down")
	svc := NewCampaignService(&mockCampaignRepo{
		getAllFunc: func(ctx context.Context) ([]models.CampaignStats, error) {
			return nil, repoErr
		},
		getByIDFunc: func(ctx context.Context, campaignID string) (*models.CampaignStats, error) {
			return nil, nil
		},
	})

	_, err := svc.GetAllCampaigns(context.Background())
	if err == nil {
		t.Fatal("GetAllCampaigns() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "get all campaigns") {
		t.Fatalf("GetAllCampaigns() error = %q, want wrapped context", err.Error())
	}
}

func TestCampaignService_GetCampaignByIDValidatesInput(t *testing.T) {
	svc := NewCampaignService(&mockCampaignRepo{
		getAllFunc: func(ctx context.Context) ([]models.CampaignStats, error) {
			return nil, nil
		},
		getByIDFunc: func(ctx context.Context, campaignID string) (*models.CampaignStats, error) {
			return nil, nil
		},
	})

	_, err := svc.GetCampaignByID(context.Background(), "")
	if err == nil {
		t.Fatal("GetCampaignByID() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "campaign id is required") {
		t.Fatalf("GetCampaignByID() error = %q, want validation error", err.Error())
	}
}

func TestCampaignService_GetCampaignByIDSuccess(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Hour)
	expected := &models.CampaignStats{CampaignID: "cmp-2", HourBucket: now}

	svc := NewCampaignService(&mockCampaignRepo{
		getAllFunc: func(ctx context.Context) ([]models.CampaignStats, error) {
			return nil, nil
		},
		getByIDFunc: func(ctx context.Context, campaignID string) (*models.CampaignStats, error) {
			if campaignID != "cmp-2" {
				t.Fatalf("unexpected campaignID = %q", campaignID)
			}
			return expected, nil
		},
	})

	got, err := svc.GetCampaignByID(context.Background(), "cmp-2")
	if err != nil {
		t.Fatalf("GetCampaignByID() error = %v", err)
	}
	if got == nil || got.CampaignID != expected.CampaignID || !got.HourBucket.Equal(expected.HourBucket) {
		t.Fatalf("GetCampaignByID() got = %#v, want %#v", got, expected)
	}
}

func TestCampaignService_GetCampaignByIDWrapsRepoError(t *testing.T) {
	repoErr := errors.New("not found")
	svc := NewCampaignService(&mockCampaignRepo{
		getAllFunc: func(ctx context.Context) ([]models.CampaignStats, error) {
			return nil, nil
		},
		getByIDFunc: func(ctx context.Context, campaignID string) (*models.CampaignStats, error) {
			return nil, repoErr
		},
	})

	_, err := svc.GetCampaignByID(context.Background(), "cmp-404")
	if err == nil {
		t.Fatal("GetCampaignByID() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "get campaign by id") {
		t.Fatalf("GetCampaignByID() error = %q, want wrapped context", err.Error())
	}
}
