package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	DbUrl string `mapstructure:"DB_URL"`
	Port  string `mapstructure:"PORT"`
}

func LoadConfig() (*Config, error) {
	viper.SetConfigFile(".env")
	_ = viper.ReadInConfig()
	viper.AutomaticEnv()

	var c Config
	if err := viper.Unmarshal(&c); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if err := c.validate(); err != nil {
		return nil, err
	}

	return &c, nil
}

func (c *Config) validate() error {
	if c.DbUrl == "" {
		return fmt.Errorf("DB_URL is required")
	}
	if c.Port == "" {
		return fmt.Errorf("PORT is required")
	}

	return nil
}
