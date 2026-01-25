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

	// Fraud Lambda
	LambdaEndpoint string `mapstructure:"LAMBDA_ENDPOINT"`
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

	return nil
}
