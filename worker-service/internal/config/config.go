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

	// AWS S3
	AWSRegion    string `mapstructure:"AWS_REGION"`
	AWSAccessKey string `mapstructure:"AWS_ACCESS_KEY_ID"`
	AWSSecretKey string `mapstructure:"AWS_SECRET_ACCESS_KEY"`
	S3Bucket     string `mapstructure:"S3_BUCKET"`

	// Archive
	ArchiveBackend          string `mapstructure:"ARCHIVE_BACKEND"`
	ArchiveBatchSize        int    `mapstructure:"ARCHIVE_BATCH_SIZE"`
	ArchiveFlushIntervalSec int    `mapstructure:"ARCHIVE_FLUSH_INTERVAL_SEC"`
	ArchivePrefixMode       string `mapstructure:"ARCHIVE_PREFIX_MODE"`

	// Fraud scoring
	FraudIPWindowSec int `mapstructure:"FRAUD_IP_WINDOW_SEC"`
	FraudIPThreshold int `mapstructure:"FRAUD_IP_THRESHOLD"`
}

func LoadConfig() (*Config, error) {
	viper.SetConfigFile(".env")
	_ = viper.ReadInConfig()
	viper.AutomaticEnv()
	for _, key := range []string{
		"DB_URL", "PORT",
		"KAFKA_BROKERS", "KAFKA_GROUP_ID", "KAFKA_TOPICS",
		"BATCH_SIZE", "FLUSH_INTERVAL_SEC",
		"REDIS_ADDR",
		"MINIO_ENDPOINT", "MINIO_ACCESS_KEY", "MINIO_SECRET_KEY", "MINIO_BUCKET",
		"AWS_REGION", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "S3_BUCKET",
		"ARCHIVE_BACKEND",
		"ARCHIVE_BATCH_SIZE", "ARCHIVE_FLUSH_INTERVAL_SEC", "ARCHIVE_PREFIX_MODE",
		"FRAUD_IP_WINDOW_SEC", "FRAUD_IP_THRESHOLD",
	} {
		if err := viper.BindEnv(key); err != nil {
			return nil, fmt.Errorf("bind env %s: %w", key, err)
		}
	}

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
	if c.ArchiveBackend == "" {
		c.ArchiveBackend = "minio"
	}
	if c.ArchiveFlushIntervalSec <= 0 {
		c.ArchiveFlushIntervalSec = 5
	}
	if c.ArchivePrefixMode == "" {
		c.ArchivePrefixMode = "status"
	}
	if c.AWSRegion == "" {
		c.AWSRegion = "ap-south-1"
	}
	if c.S3Bucket == "" {
		c.S3Bucket = c.MinIOBucket
	}
	if c.FraudIPWindowSec <= 0 {
		c.FraudIPWindowSec = 300
	}
	if c.FraudIPThreshold <= 0 {
		c.FraudIPThreshold = 100
	}

	return nil
}
