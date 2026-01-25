package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
	"worker-service/internal/archive"
	"worker-service/internal/config"
	"worker-service/internal/db"
	"worker-service/internal/dedupe"
	"worker-service/internal/fraud"
	"worker-service/internal/kafka"
	"worker-service/internal/repo"
	"worker-service/internal/service"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Configuration loaded successfully")

	// Initialize DB
	DB, err := db.InitDB(cfg.DBUrl)
	if err != nil {
		log.Fatal("failed to initialize DB:", err)
	}
	defer DB.Close()

	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(5)
	DB.SetConnMaxLifetime(5 * time.Minute)

	log.Println("DB initialized successfully")

	// Initialize MinIO Archiver
	archiver, err := archive.NewMinIOArchiver(
		cfg.MinIOEndpoint,
		cfg.MinIOAccessKey,
		cfg.MinIOSecretKey,
		cfg.MinIOBucket,
	)
	if err != nil {
		log.Fatal("failed to initialize MinIO archiver:", err)
	}
	log.Println("MinIO archiver initialized successfully")

	// Initialize Redis Deduplicator
	dedup, err := dedupe.NewRedisDeduplicator(cfg.RedisAddr, 24*time.Hour)
	if err != nil {
		log.Fatal("failed to initialize Redis deduplicator:", err)
	}
	log.Println("Redis deduplicator initialized successfully")

	// Initialize Fraud Lambda Client
	fraudClient := fraud.NewLambdaClient(cfg.LambdaEndpoint)
	log.Println("Fraud Lambda client initialized successfully")

	// Initialize Repository
	eventRepo := repo.NewEventRepo(DB)
	log.Println("Repository layer initialized")

	// Initialize Event Service with Smart Worker chain
	eventService := service.NewEventService(eventRepo, service.Config{
		Archiver:   archiver,
		Dedup:      dedup,
		FraudCheck: fraudClient,
	})
	log.Println("Service layer initialized with Smart Worker chain")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	kafkaConsumer, err := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers: cfg.GetKafkaBrokers(),
		GroupID: cfg.KafkaGroupID,
		Topics:  cfg.GetKafkaTopics(),
	}, eventService)
	if err != nil {
		log.Fatal("failed to create kafka consumer:", err)
	}
	defer kafkaConsumer.Close()

	log.Printf("kafka consumer created.")

	if err := kafkaConsumer.Start(ctx); err != nil {
		log.Fatalf("Failed to start kafka consumer: %v", err)
	}
	log.Println("Kafka consumer started successfully.")

	go metricsReporter(ctx, eventService)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	log.Println("===========================================")
	log.Println("🚀 Smart Worker Service is running...")
	log.Println("Processing chain: Archive → Dedupe → Fraud Check → DB")
	log.Println("===========================================")

	// Wait for shutdown signal
	<-sigCh

	log.Println("\n===========================================")
	log.Println("Shutdown signal received. Shutting down gracefully...")
	log.Println("=============================================")

	cancel()

	shutdownCtx, shutDownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutDownCancel()

	// Shutdown service
	log.Println("Shutting down service...")
	if err := eventService.Shutdown(shutdownCtx); err != nil {
		log.Printf("Warning: Service shutdown error: %v", err)
	} else {
		log.Println("Service shutdown complete")
	}

	// Close kafka consumer
	log.Println("Closing kafka consumer...")
	if err := kafkaConsumer.Close(); err != nil {
		log.Printf("Warning: Kafka consumer close error: %v", err)
	} else {
		log.Println("Kafka consumer closed")
	}

	// print final metrics
	metrics := eventService.GetMetrics()
	log.Println("\n===========================================")
	log.Println("📊 Final Metrics:")
	log.Printf("   Total Processed:    %d events", metrics.TotalProcessed)
	log.Printf("   Total Archived:     %d events", metrics.TotalArchived)
	log.Printf("   Total Duplicates:   %d events", metrics.TotalDuplicates)
	log.Printf("   Total Fraud:        %d events", metrics.TotalFraudDetected)
	log.Printf("   Total Failed:       %d events", metrics.TotalFailed)
	log.Printf("   Avg Processing:     %v per event", metrics.AvgProcessingTime)
	if metrics.TotalProcessed > 0 {
		successRate := float64(metrics.TotalProcessed) / float64(metrics.TotalProcessed+metrics.TotalFailed) * 100
		log.Printf("   Success Rate:       %.2f%%", successRate)
	}
	log.Println("===========================================")

	log.Println("Shutdown complete. Goodbye!")
}

func metricsReporter(ctx context.Context, svc *service.EventService) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			metrics := svc.GetMetrics()
			log.Println("-------------------------------------------")
			log.Println("📊 Service Metrics:")
			log.Printf("   Processed:      %d events", metrics.TotalProcessed)
			log.Printf("   Archived:       %d events", metrics.TotalArchived)
			log.Printf("   Duplicates:     %d events", metrics.TotalDuplicates)
			log.Printf("   Fraud Detected: %d events", metrics.TotalFraudDetected)
			log.Printf("   Failed:         %d events", metrics.TotalFailed)
			log.Printf("   Avg Time:       %v per event", metrics.AvgProcessingTime)
			if !metrics.LastProcessedAt.IsZero() {
				log.Printf("   Last Processed: %v", metrics.LastProcessedAt.Format(time.RFC3339))
			}
			log.Println("-------------------------------------------")

		case <-ctx.Done():
			log.Println("Metrics reporter stopped.")
			return
		}
	}
}
