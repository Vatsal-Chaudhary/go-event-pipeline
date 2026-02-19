package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	DBUrl string `mapstructure:"DB_URL"`
	Port  string `mapstructure:"PORT"`

	// Kafka
	KafkaBrokers string `mapstructure:"KAFKA_BROKERS"`
	KafkaGroupID string `mapstructure:"KAFKA_GROUP_ID"`
	KafkaTopics  string `mapstructure:"KAFKA_TOPICS"`

	// Service Configuration
	BatchSize        int `mapstructure:"BATCH_SIZE"`
	FlushIntervalSec int `mapstructure:"FLUSH_INTERVAL_SEC"`

	// Redis
	RedisAddr string `mapstructure:"REDIS_ADDR"`

	// MinIO
	MinIOEndpoint  string `mapstructure:"MINIO_ENDPOINT"`
	MinIOAccessKey string `mapstructure:"MINIO_ACCESS_KEY"`
	MinIOSecretKey string `mapstructure:"MINIO_SECRET_KEY"`
	MinIOBucket    string `mapstructure:"MINIO_BUCKET"`

	// Archive
	ArchiveBatchSize        int    `mapstructure:"ARCHIVE_BATCH_SIZE"`
	ArchiveFlushIntervalSec int    `mapstructure:"ARCHIVE_FLUSH_INTERVAL_SEC"`
	ArchivePrefixMode       string `mapstructure:"ARCHIVE_PREFIX_MODE"`

	// Fraud scoring
	FraudMode        string `mapstructure:"FRAUD_MODE"`
	FraudEndpoint    string `mapstructure:"FRAUD_ENDPOINT"`
	FraudIPWindowSec int    `mapstructure:"FRAUD_IP_WINDOW_SEC"`
	FraudIPThreshold int    `mapstructure:"FRAUD_IP_THRESHOLD"`
}

func LoadConfig() (*Config, error) {
	viper.SetConfigFile(".env")
	_ = viper.ReadInConfig()
	viper.AutomaticEnv()

	var c Config
	err := viper.Unmarshal(&c)
	if err != nil {
		fmt.Println("Error in unmarshalling from env to config")
		return nil, err
	}

	c.validate()

	return &c, nil
}

func (c *Config) GetKafkaBrokers() []string {
	return strings.Split(c.KafkaBrokers, ",")
}

func (c *Config) GetKafkaTopics() []string {
	return strings.Split(c.KafkaTopics, ",")
}

func (c *Config) validate() error {
	if c.DBUrl == "" {
		return fmt.Errorf("DB_URL is required")
	}
	if c.KafkaTopics == "" {
		return fmt.Errorf("KAFKA_TOPICS is required")
	}
	if c.Port == "" {
		return fmt.Errorf("PORT is required")
	}
	if c.ArchiveBatchSize <= 0 {
		c.ArchiveBatchSize = 100
	}
	if c.ArchiveFlushIntervalSec <= 0 {
		c.ArchiveFlushIntervalSec = 5
	}
	if c.ArchivePrefixMode == "" {
		c.ArchivePrefixMode = "status"
	}
	if c.FraudMode == "" {
		c.FraudMode = "inprocess"
	}
	if c.FraudIPWindowSec <= 0 {
		c.FraudIPWindowSec = 300
	}
	if c.FraudIPThreshold <= 0 {
		c.FraudIPThreshold = 100
	}

	return nil
}
