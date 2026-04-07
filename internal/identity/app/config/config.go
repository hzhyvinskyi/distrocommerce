package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	App  AppConfig
	HTTP HTTPConfig
}

type AppConfig struct {
	Name        string
	Version     string
	Environment string
	LogLevel    string
}

type HTTPConfig struct {
	Port            int
	ShutdownTimeout time.Duration
}

func Load() (*Config, error) {
	_ = godotenv.Load("configs/identity-sv.env", ".env")

	cfg := &Config{}
	if err := cfg.parse(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) parse() error {
	c.App.Name = requireEnv("APP_NAME")
	c.App.Version = getEnv("APP_VERSION", "dev")
	c.App.Environment = getEnv("APP_ENV", "local")
	c.App.LogLevel = getEnv("APP_LOG_LEVEL", "info")

	c.HTTP.Port = getEnvInt("HTTP_PORT", 8087)
	c.HTTP.ShutdownTimeout = getEnvDuration("HTTP_SHUTDOWN_TIMEOUT", 30*time.Second)

	return nil
}

func requireEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("Required environment variable %s is not set", key)
	}

	return val
}

func getEnv(key, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}

	return val
}

func getEnvInt(key string, fallback int) int {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}

	n, err := strconv.Atoi(val)
	if err != nil {
		log.Fatalf("env var %q must be integer, got %q", key, val)
	}

	return n
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}

	d, err := time.ParseDuration(val)
	if err != nil {
		log.Fatalf("env var %q must be duration, got %q", key, val)
	}

	return d
}
