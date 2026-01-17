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

	if c.DBUrl == "" {
		return nil, fmt.Errorf("DB_URL is required")
	}

	return &c, nil
}

func (c *Config) GetKafkaBrokers() []string {
	if c.KafkaBrokers == "" {
		return []string{"localhost:9092"}
	}
	return strings.Split(c.KafkaBrokers, ",")
}

func (c *Config) GetKafkaTopics() []string {
	if c.KafkaTopics == "" {
		return []string{"events"}
	}
	return strings.Split(c.KafkaTopics, ",")
}
