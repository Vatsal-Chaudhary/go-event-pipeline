package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"query-service/internal/config"
	"query-service/internal/db"
	"query-service/internal/handler"
	"query-service/internal/repo"
	"query-service/internal/service"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	database, err := db.InitDB(cfg.DbUrl)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	database.SetMaxOpenConns(25)
	database.SetMaxIdleConns(5)
	database.SetConnMaxLifetime(5 * time.Minute)

	campaignRepo := repo.NewCampaignRepo(database)
	campaignService := service.NewCampaignService(campaignRepo)
	campaignHandler := handler.NewCampaignHandler(campaignService)

	app := fiber.New(fiber.Config{
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	})

	app.Use(recover.New())
	app.Use(logger.New())

	app.Get("/health", handler.HealthCheck)
	app.Get("/campaigns", campaignHandler.GetAllCampaigns)
	app.Get("/campaigns/:id", campaignHandler.GetCampaignByID)

	go func() {
		if err := app.Listen(":" + cfg.Port); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	if err := app.Shutdown(); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}
	log.Println("Server stopped")
}
