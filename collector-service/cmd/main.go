package main

import (
	"collector-service/internal/config"
	"collector-service/internal/handler"
	"collector-service/internal/kafka"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	producer, err := kafka.NewProducer(kafka.ProducerConfig{
		Brokers: cfg.GetKafkaBrokers(),
		Topic:   cfg.KafkaTopic,
	})
	if err != nil {
		log.Fatalf("failed to create producer: %v", err)
	}

	defer func() {
		if err := producer.Close(); err != nil {
			log.Printf("Error closing producer: %v", err)
		}
	}()

	eventHandler := handler.NewEventHandler(producer)

	app := fiber.New(fiber.Config{
		DisableStartupMessage: false,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{"error": err.Error()})
		},
	})

	app.Use(recover.New())
	app.Use(logger.New())

	app.Post("/event", eventHandler.HandleEvent)
	app.Post("/events/batch", eventHandler.HandleEventBatch)
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Shutting down...")
		app.Shutdown()
	}()

	log.Printf("🚀 Collector running on %s", cfg.Port)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
