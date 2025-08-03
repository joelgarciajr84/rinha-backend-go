package config

import (
	"os"
	"strconv"
)

type ApplicationSettings struct {
	PrimaryProcessorURL  string
	FallbackProcessorURL string
	ServerPort           string
	RedisConnectionURL   string
	WorkerPoolSize       int
	QueueCapacity        int
	ConcurrencyLimit     int
}

func LoadEnvironmentConfig() *ApplicationSettings {
	return &ApplicationSettings{
		PrimaryProcessorURL:  getEnvironmentVariable("PAYMENT_PROCESSOR_DEFAULT_URL", "http://localhost:8001"),
		FallbackProcessorURL: getEnvironmentVariable("PAYMENT_PROCESSOR_FALLBACK_URL", "http://localhost:8002"),
		ServerPort:           getEnvironmentVariable("PORT", ":9999"),
		RedisConnectionURL:   getEnvironmentVariable("REDIS_URL", "127.0.0.1:6379"),
		WorkerPoolSize:       getIntegerEnvironmentVariable("WORKERS", 21),
		QueueCapacity:        12000,
		ConcurrencyLimit:     200000,
	}
}

func getEnvironmentVariable(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntegerEnvironmentVariable(key string, defaultValue int) int {
	if valueStr := os.Getenv(key); valueStr != "" {
		if value, err := strconv.Atoi(valueStr); err == nil {
			return value
		}
	}
	return defaultValue
}
