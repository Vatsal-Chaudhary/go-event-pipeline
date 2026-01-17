package service

import (
	"context"
	"fmt"

	"query-service/internal/models"
	"query-service/internal/repo/interfaces"
)

type CampaignService struct {
	repo interfaces.CampaignRepo
}

func NewCampaignService(repo interfaces.CampaignRepo) *CampaignService {
	return &CampaignService{repo: repo}
}

func (s *CampaignService) GetAllCampaigns(ctx context.Context) ([]models.CampaignStats, error) {
	campaigns, err := s.repo.GetAllCampaigns(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all campaigns: %w", err)
	}
	return campaigns, nil
}

func (s *CampaignService) GetCampaignByID(ctx context.Context, campaignID string) (*models.CampaignStats, error) {
	if campaignID == "" {
		return nil, fmt.Errorf("campaign id is required")
	}

	campaign, err := s.repo.GetCampaignByID(ctx, campaignID)
	if err != nil {
		return nil, fmt.Errorf("get campaign by id: %w", err)
	}
	return campaign, nil
}
