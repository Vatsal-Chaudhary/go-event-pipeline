package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Port         string `mapstructure:"PORT"`
	KafkaBrokers string `mapstructure:"KAFKA_BROKERS"`
	KafkaTopic   string `mapstructure:"KAFKA_TOPIC"`
}

func LoadConfig() (*Config, error) {
	viper.SetConfigFile(".env")
	_ = viper.ReadInConfig()

	viper.AutomaticEnv()

	// Set default topic
	viper.SetDefault("KAFKA_TOPIC", "raw-events")

	var c Config
	if err := viper.Unmarshal(&c); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if c.KafkaTopic == "" {
		return nil, fmt.Errorf("KAFKA_TOPIC is required")
	}
	if len(c.KafkaBrokers) == 0 {
		return nil, fmt.Errorf("KAFKA_BROKERS is required")
	}

	return &c, nil
}

func (c *Config) GetKafkaBrokers() []string {
	brokers := strings.Split(c.KafkaBrokers, ",")

	for i := range brokers {
		brokers[i] = strings.TrimSpace(brokers[i])
	}
	return brokers
}
