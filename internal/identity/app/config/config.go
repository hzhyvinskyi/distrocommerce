package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	App      AppConfig
	HTTP     HTTPConfig
	Postgres PostgresConfig
	Redis    RedisConfig
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

type PostgresConfig struct {
	Host                  string
	Port                  int
	User                  string
	Password              string
	Database              string
	SSLMode               string
	MaxConns              int
	MinConns              int
	MaxConnLifetime       time.Duration
	MaxConnLifetimeJitter time.Duration
	MaxConnIdleTime       time.Duration
	HealthCheckPeriod     time.Duration
}

func (c PostgresConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.Database, c.SSLMode,
	)
}

type RedisConfig struct {
	Addr     string
	Password string
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

	c.Postgres.Host = getEnv("POSTGRES_HOST", "localhost")
	c.Postgres.Port = getEnvInt("POSTGRES_PORT", 5432)
	c.Postgres.User = requireEnv("POSTGRES_USER")
	c.Postgres.Password = requireEnv("POSTGRES_PASSWORD")
	c.Postgres.Database = requireEnv("POSTGRES_DB")
	c.Postgres.SSLMode = getEnv("POSTGRES_SSL_MODE", "disable")
	c.Postgres.MaxConns = getEnvInt("POSTGRES_MAX_CONNS", 10)
	c.Postgres.MinConns = getEnvInt("POSTGRES_MIN_CONNS", 0)
	c.Postgres.MaxConnLifetime = getEnvDuration("POSTGRES_MAX_CONN_LIFETIME", 30*time.Minute)
	c.Postgres.MaxConnLifetimeJitter = getEnvDuration("POSTGRES_MAX_CONN_JITTER", 5*time.Minute)
	c.Postgres.MaxConnIdleTime = getEnvDuration("POSTGRES_MAX_CONN_IDLE_TIME", 5*time.Minute)
	c.Postgres.HealthCheckPeriod = getEnvDuration("POSTGRES_HEALTH_CHECK_PERIOD", 30*time.Second)

	c.Redis.Addr = getEnv("REDIS_ADDR", "localhost:6379")
	c.Redis.Password = getEnv("REDIS_PASSWORD", "")

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
