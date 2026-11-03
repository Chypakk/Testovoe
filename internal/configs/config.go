package configs

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type ServerConfig struct {
	Port string `env:"SERVER_PORT" envDefault:"8080"`
}

type DatabaseConfig struct {
	Driver   string `env:"DB_DRIVER" envDefault:"postgres"`
	Host     string `env:"DB_HOST" envDefault:"localhost"`
	Port     string `env:"DB_PORT" envDefault:"5432"`
	User     string `env:"DB_USER" envDefault:"postgres"`
	Password string `env:"DB_PASSWORD" envDefault:"postgres"`
	Name     string `env:"DB_NAME" envDefault:"testovoe"`
	SSLMode  string `env:"DB_SSLMODE" envDefault:"disable"`
}

func (d DatabaseConfig) NormalizedDriver() string {
	driver := strings.ToLower(strings.TrimSpace(d.Driver))
	if driver == "postgresql" {
		driver = "postgres"
	}
	return driver
}

type RedisConfig struct {
	Host     string `env:"REDIS_HOST" envDefault:"localhost"`
	Port     string `env:"REDIS_PORT" envDefault:"6379"`
	Password string `env:"REDIS_PASSWORD" envDefault:""`
	DB       int    `env:"REDIS_DB" envDefault:"0"`
}

type JWTConfig struct {
	Secret string `env:"JWT_SECRET"`
}

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
}

func LoadConfig() (*Config, error) {
	if err := godotenv.Load(".env"); err != nil {
		log.Printf("Warning: failed to load .env file: %v", err)
	}

	cfg := &Config{}

	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse env: %w", err)
	}

	cfg.Database.Driver = cfg.Database.NormalizedDriver()

	switch cfg.Database.Driver {
	case "postgres", "mysql":
	default:
		return nil, fmt.Errorf("unsupported DB_DRIVER: %s", cfg.Database.Driver)
	}

	if strings.TrimSpace(cfg.JWT.Secret) == "" {
		return nil, errors.New("JWT_SECRET is required")
	}

	if strings.TrimSpace(cfg.Server.Port) == "" {
		cfg.Server.Port = "8080"
	}

	return cfg, nil
}
