package config

import (
	"fmt"
	"strings"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	Environment              string `env:"ENVIRONMENT,required"`
	LineMessageChannelSecret string `env:"LINE_MESSAGE_CHANNEL_SECRET,required"`
	LineMessageChannelToken  string `env:"LINE_MESSAGE_CHANNEL_TOKEN,required"`
	DBHost                   string `env:"DB_HOST,required"`
	DBPort                   string `env:"DB_PORT,required"`
	DBUser                   string `env:"DB_USER,required"`
	DBPassword               string `env:"DB_PASSWORD,required"`
	DBName                   string `env:"DB_NAME,required"`
}

func (c *Config) IsProduction() bool {
	return strings.ToLower(c.Environment) == "production"
}

func (c *Config) DatabaseURL() string {
	return fmt.Sprintf("postgresql://%s:%s@%s:%s/%s",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName)
}

func Load() (*Config, error) {
	// Load .env file if present (ignored in production / when file doesn't exist)
	_ = godotenv.Load()

	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	return cfg, nil
}
