package handler

import (
	"github.com/gofiber/fiber/v2"

	"query-service/internal/service"
)

type CampaignHandler struct {
	service *service.CampaignService
}

func NewCampaignHandler(svc *service.CampaignService) *CampaignHandler {
	return &CampaignHandler{service: svc}
}

func (h *CampaignHandler) GetAllCampaigns(c *fiber.Ctx) error {
	campaigns, err := h.service.GetAllCampaigns(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to fetch campaigns",
		})
	}

	return c.JSON(fiber.Map{
		"data": campaigns,
	})
}

func (h *CampaignHandler) GetCampaignByID(c *fiber.Ctx) error {
	campaignID := c.Params("id")
	if campaignID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "campaign id is required",
		})
	}

	campaign, err := h.service.GetCampaignByID(c.Context(), campaignID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to fetch campaign",
		})
	}

	if campaign == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "campaign not found",
		})
	}

	return c.JSON(fiber.Map{
		"data": campaign,
	})
}

func HealthCheck(c *fiber.Ctx) error {
	return c.SendString("OK")
}
