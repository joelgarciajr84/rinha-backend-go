package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq"
)

type DatabaseConfig struct {
	Pool *pgxpool.Pool
}

func NewDatabaseConfig() *DatabaseConfig {
	pool, err := createConnectionPool()
	if err != nil {
		log.Fatalf("Failed to create database connection pool: %v", err)
	}

	if err := testConnection(pool); err != nil {
		log.Fatalf("Database connection test failed: %v", err)
	}

	log.Println("Database connection established successfully")
	return &DatabaseConfig{Pool: pool}
}

func createConnectionPool() (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(buildConnectionString())
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	config.MaxConns = 300
	config.MinConns = 10
	config.MaxConnLifetime = time.Minute * 30
	config.MaxConnIdleTime = time.Minute * 5

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	return pool, nil
}

func testConnection(pool *pgxpool.Pool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var result int
	err := pool.QueryRow(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}

	if result != 1 {
		return fmt.Errorf("unexpected result from connection test: %d", result)
	}

	return nil
}

func buildConnectionString() string {
	host := getEnvWithDefault("DB_HOST", "financial-db:5432")
	user := getEnvWithDefault("DB_USER", "root")
	password := getEnvWithDefault("DB_PASS", "root")
	dbname := getEnvWithDefault("DB_NAME", "transactions")

	connStr := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable",
		user, password, host, dbname)

	return connStr
}

func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
