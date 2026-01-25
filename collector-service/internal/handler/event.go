package handler

import (
	"collector-service/internal/kafka"
	"collector-service/internal/models"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

type EventHandler struct {
	producer *kafka.Producer
}

func NewEventHandler(producer *kafka.Producer) *EventHandler {
	return &EventHandler{producer: producer}
}

// extractIPAddress extracts the client IP with priority: X-Forwarded-For -> X-Real-IP -> RemoteAddr
func extractIPAddress(c *fiber.Ctx) string {
	// Priority 1: Try X-Forwarded-For header first
	xff := c.Get("X-Forwarded-For")
	if xff != "" {
		// X-Forwarded-For can contain multiple IPs: "client, proxy1, proxy2"
		// Take the first one (leftmost = original client)
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Priority 2: Try X-Real-IP header
	xRealIP := c.Get("X-Real-IP")
	if xRealIP != "" {
		return strings.TrimSpace(xRealIP)
	}

	// Priority 3: Fallback to RemoteAddr
	ip := c.IP()
	return ip
}

func (h *EventHandler) HandleEvent(c *fiber.Ctx) error {
	var event models.RawEvent
	if err := c.BodyParser(&event); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid Request Body",
		})
	}

	// Basic validation
	if event.EventType == "" || event.UserID == "" || event.CampaignID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "event_type, user_id and campaign_id are required",
		})
	}

	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}

	// Extract and attach IP address
	event.IPAddress = extractIPAddress(c)

	if err := h.producer.SendEvent(c.Context(), event); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to queue event",
		})
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"status": "queued",
	})
}

func (h *EventHandler) HandleEventBatch(c *fiber.Ctx) error {
	var events []models.RawEvent

	if err := c.BodyParser(&events); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid Request Body",
		})
	}
	if len(events) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "At least one event is required",
		})
	}
	if len(events) > 1000 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Maximum 1000 events per batch",
		})
	}

	now := time.Now()
	ip := extractIPAddress(c)

	for i := range events {
		if events[i].CreatedAt.IsZero() {
			events[i].CreatedAt = now
		}
		// Attach IP address to each event in batch
		events[i].IPAddress = ip
	}

	if err := h.producer.SendEventBatch(c.Context(), events); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to queue events.",
		})
	}
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"status": "queued",
		"count":  len(events),
	})
}
