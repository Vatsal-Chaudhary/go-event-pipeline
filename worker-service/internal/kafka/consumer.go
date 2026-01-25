package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"worker-service/internal/models"
	"worker-service/internal/service"

	"github.com/IBM/sarama"
)

type Consumer struct {
	service       *service.EventService
	consumerGroup sarama.ConsumerGroup
	topics        []string
	ready         chan bool
}

type ConsumerConfig struct {
	Brokers []string
	GroupID string
	Topics  []string
}

func NewConsumer(config ConsumerConfig, eventService *service.EventService) (*Consumer, error) {
	saramaConfig := sarama.NewConfig()
	saramaConfig.Version = sarama.V2_6_0_0
	saramaConfig.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
	saramaConfig.Consumer.Offsets.Initial = sarama.OffsetNewest
	saramaConfig.Consumer.Return.Errors = true

	consumerGroup, err := sarama.NewConsumerGroup(config.Brokers, config.GroupID, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("create a consumer group: %w", err)
	}

	return &Consumer{
		service:       eventService,
		consumerGroup: consumerGroup,
		topics:        config.Topics,
		ready:         make(chan bool),
	}, nil
}

func (c *Consumer) Start(ctx context.Context) error {
	handler := &consumerGroupHandler{
		consumer: c,
		ready:    c.ready,
	}

	go func() {
		for {
			if err := c.consumerGroup.Consume(ctx, c.topics, handler); err != nil {
				log.Printf("Consumer error: %v", err)
			}

			if ctx.Err() != nil {
				return
			}
		}
	}()

	<-c.ready
	log.Println("Kafka consumer started")
	return nil
}

func (c *Consumer) Close() error {
	return c.consumerGroup.Close()
}

type consumerGroupHandler struct {
	consumer *Consumer
	ready    chan bool
}

func (h *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	close(h.ready)
	return nil
}

func (h *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claims sarama.ConsumerGroupClaim) error {
	for {
		select {
		case message := <-claims.Messages():
			if message == nil {
				return nil
			}

			var kafkaMsg models.KafkaEventMessage
			if err := json.Unmarshal(message.Value, &kafkaMsg); err != nil {
				log.Printf("Unmarshal error: %v", err)
				session.MarkMessage(message, "")
				continue
			}

			// Process single event through Smart Worker chain
			if err := h.consumer.service.ProcessEvent(session.Context(), &kafkaMsg); err != nil {
				log.Printf("Process event error: %v", err)
				// Still mark message to avoid reprocessing
				session.MarkMessage(message, "")
			} else {
				session.MarkMessage(message, "")
			}

		case <-session.Context().Done():
			return nil
		}
	}
}
