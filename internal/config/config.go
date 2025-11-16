package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	JWT       JWTConfig
	Logger    LoggerConfig
	Redis     RedisConfig
	RateLimit RateLimitConfig
}

type ServerConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	Environment  string
}

type DatabaseConfig struct {
	URI      string
	Name     string
	Timeout  time.Duration
	MaxConns int
}

type JWTConfig struct {
	Secret          string
	ExpirationHours int
}

type LoggerConfig struct {
	Level string
}

type RedisConfig struct {
	URL      string
	Password string
	DB       int
}

type RateLimitConfig struct {
	Enabled          bool
	Requests         int
	Window           time.Duration
	CriticalPaths    []string
	CriticalRequests int
	CriticalWindow   time.Duration
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		// It's okay if .env doesn't exist in production
	}

	config := &Config{
		Server: ServerConfig{
			Port:         getEnv("PORT", "5000"),
			ReadTimeout:  getEnvAsDuration("SERVER_READ_TIMEOUT", "10s"),
			WriteTimeout: getEnvAsDuration("SERVER_WRITE_TIMEOUT", "10s"),
			Environment:  getEnv("ENVIRONMENT", "development"),
		},
		Database: DatabaseConfig{
			URI:      getEnv("MONGO_DB_URL", "mongodb://localhost:27017"),
			Name:     getEnv("MONGO_DB_NAME", "hotel-reservation"),
			Timeout:  getEnvAsDuration("DB_TIMEOUT", "5s"),
			MaxConns: getEnvAsInt("DB_MAX_CONNECTIONS", 100),
		},
		JWT: JWTConfig{
			Secret:          getEnv("JWT_SECRET", "your-secret-key"),
			ExpirationHours: getEnvAsInt("JWT_EXPIRATION_HOURS", 4),
		},
		Logger: LoggerConfig{
			Level: getEnv("LOG_LEVEL", "info"),
		},
		Redis: RedisConfig{
			URL:      getEnv("REDIS_URL", "redis://localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvAsInt("REDIS_DB", 0),
		},
		RateLimit: RateLimitConfig{
			Enabled:          getEnvAsBool("RATE_LIMIT_ENABLED", true),
			Requests:         getEnvAsInt("RATE_LIMIT_REQUESTS", 60),
			Window:           getEnvAsDuration("RATE_LIMIT_WINDOW", "1m"),
			CriticalPaths:    getEnvAsSlice("RATE_LIMIT_CRITICAL_PATHS", []string{"/api/v1/room/:id/book", "/api/v1/booking"}),
			CriticalRequests: getEnvAsInt("RATE_LIMIT_CRITICAL_REQUESTS", 20),
			CriticalWindow:   getEnvAsDuration("RATE_LIMIT_CRITICAL_WINDOW", "1m"),
		},
	}

	return config, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsDuration(key string, defaultValue string) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	duration, _ := time.ParseDuration(defaultValue)
	return duration
}

func getEnvAsBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		switch strings.ToLower(value) {
		case "true", "1", "yes", "y":
			return true
		case "false", "0", "no", "n":
			return false
		}
	}
	return defaultValue
}

func getEnvAsSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		parts := strings.Split(value, ",")
		var trimmed []string
		for _, part := range parts {
			if v := strings.TrimSpace(part); v != "" {
				trimmed = append(trimmed, v)
			}
		}
		if len(trimmed) > 0 {
			return trimmed
		}
	}
	return defaultValue
}
