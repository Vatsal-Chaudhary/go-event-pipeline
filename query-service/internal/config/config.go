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
	for _, key := range []string{"DB_URL", "PORT"} {
		if err := viper.BindEnv(key); err != nil {
			return nil, fmt.Errorf("bind env %s: %w", key, err)
		}
	}

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
