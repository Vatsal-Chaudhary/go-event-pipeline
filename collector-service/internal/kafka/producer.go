package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"collector-service/internal/models"

	"github.com/IBM/sarama"
)

type Producer struct {
	producer sarama.SyncProducer
	topic    string
}

type ProducerConfig struct {
	Brokers []string
	Topic   string
}

func NewProducer(config ProducerConfig) (*Producer, error) {
	saramaConfig := sarama.NewConfig()
	saramaConfig.Version = sarama.V2_6_0_0
	saramaConfig.Producer.RequiredAcks = sarama.WaitForAll
	saramaConfig.Producer.Retry.Max = 5
	saramaConfig.Producer.Return.Successes = true
	saramaConfig.Producer.Compression = sarama.CompressionSnappy
	saramaConfig.Producer.Partitioner = sarama.NewHashPartitioner

	producer, err := sarama.NewSyncProducer(config.Brokers, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create producer: %w", err)
	}

	log.Println("Kafka producer initialized successfully")
	return &Producer{
		producer: producer,
		topic:    config.Topic,
	}, nil
}

func (p *Producer) SendEvent(ctx context.Context, event models.RawEvent) error {
	kafkaMsg := models.KafkaEventMessage{
		EventType:  event.EventType,
		UserID:     event.UserID,
		CampaignID: event.CampaignID,
		GeoCountry: event.GeoCountry,
		DeviceType: event.DeviceType,
		Timestamp:  &event.CreatedAt,
		IPAddress:  event.IPAddress,
		MetaData:   event.MetaData,
	}

	payload, err := json.Marshal(kafkaMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	message := &sarama.ProducerMessage{
		Topic:     p.topic,
		Key:       nil, // Round-robin partitioning
		Value:     sarama.ByteEncoder(payload),
		Timestamp: time.Now(),
	}

	partition, offset, err := p.producer.SendMessage(message)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	log.Printf("Message sent to partition %d at offset %d", partition, offset)
	return nil
}

func (p *Producer) SendEventBatch(ctx context.Context, events []models.RawEvent) error {
	messages := make([]*sarama.ProducerMessage, 0, len(events))

	for _, event := range events {
		kafkaMsg := models.KafkaEventMessage{
			EventType:  event.EventType,
			UserID:     event.UserID,
			CampaignID: event.CampaignID,
			GeoCountry: event.GeoCountry,
			DeviceType: event.DeviceType,
			Timestamp:  &event.CreatedAt,
			IPAddress:  event.IPAddress,
			MetaData:   event.MetaData,
		}

		payload, err := json.Marshal(kafkaMsg)
		if err != nil {
			log.Printf("Failed to marshal event: %v", err)
			continue
		}

		messages = append(messages, &sarama.ProducerMessage{
			Topic:     p.topic,
			Key:       nil, // Round-robin partitioning
			Value:     sarama.ByteEncoder(payload),
			Timestamp: time.Now(),
		})
	}

	if len(messages) == 0 {
		return fmt.Errorf("no valid messages to send")
	}

	err := p.producer.SendMessages(messages)
	if err != nil {
		return fmt.Errorf("failed to send batch: %w", err)
	}

	log.Printf("Batch of %d messages sent successfully", len(messages))
	return nil
}

func (p *Producer) Close() error {
	if err := p.producer.Close(); err != nil {
		return fmt.Errorf("failed to close producer: %w", err)
	}
	log.Println("Kafka producer closed")
	return nil
}
