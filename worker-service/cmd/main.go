package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
	"worker-service/internal/config"
	"worker-service/internal/db"
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

	DB, err := db.InitDB(cfg.DBUrl)
	if err != nil {
		log.Fatal("failed to initialize DB:", err)
	}
	defer DB.Close()

	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(5)
	DB.SetConnMaxLifetime(5 * time.Minute)

	log.Println("DB initialized successfully")

	eventRepo := repo.NewEventRepo(DB)
	log.Println("Repository layer initialized")

	eventService := service.NewEventService(eventRepo, service.Config{
		BatchSize:     cfg.BatchSize,
		FlushInterval: time.Duration(cfg.FlushIntervalSec) * time.Second,
	})
	log.Println("Service layer initialized")

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
	log.Println("Event Worker Service is running...")
	log.Println("===========================================")

	// Wait for shutdown signal
	<-sigCh

	log.Println("\n===========================================")
	log.Println("Shutdown signal received. Shutting down gracefully...")
	log.Println("=============================================")

	cancel()

	shutdownCtx, shutDownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutDownCancel()

	// Shutdown service (flushes remaining buffer)
	log.Println("Flushing remaining events...")
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
	log.Println("Final Metrics:")
	log.Printf(" Total Processed: %d events", metrics.TotalProcessed)
	log.Printf(" Total Failed: %d failed", metrics.TotalFailed)
	log.Printf(" Total Batches: %d batches", metrics.TotalBatches)
	log.Printf(" Avergae Processing: %d per event", metrics.AvgProcessingTime)
	if metrics.TotalProcessed > 0 {
		successRate := float64(metrics.TotalProcessed) / float64(metrics.TotalProcessed+metrics.TotalFailed) * 100
		log.Printf(" Success Rate: %.2f%%", successRate)
	}
	log.Println("\n==========================================")

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
			log.Printf("   Processed: %d events", metrics.TotalProcessed)
			log.Printf("   Failed:    %d events", metrics.TotalFailed)
			log.Printf("   Batches:   %d", metrics.TotalBatches)
			log.Printf("   Avg Time:  %v per event", metrics.AvgProcessingTime)
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
