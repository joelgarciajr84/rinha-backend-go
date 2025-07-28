package infrastructure

import (
	"context"
	"fmt"
	"galo/internal/domain"
	"math"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisMetricsRepository struct {
	client *redis.Client
}

func NewRedisMetricsRepository(redisURL string) *RedisMetricsRepository {
	client := redis.NewClient(&redis.Options{
		Addr:     redisURL,
		Password: "",
		DB:       0,
	})

	return &RedisMetricsRepository{
		client: client,
	}
}

func (r *RedisMetricsRepository) StoreTransactionData(processorType string, transaction domain.TransactionRequest) error {
	ctx := context.Background()

	timestamp, err := time.Parse("2006-01-02T15:04:05.000Z07:00", transaction.SubmittedAt)
	if err != nil {
		return fmt.Errorf("erro ao converter timestamp: %w", err)
	}

	pipeline := r.client.Pipeline()

	pipeline.HSet(ctx, fmt.Sprintf("metrics:%s:data", processorType),
		transaction.IdentificationCode, transaction.MonetaryValue)

	pipeline.ZAdd(ctx, fmt.Sprintf("metrics:%s:timeline", processorType), redis.Z{
		Score:  float64(timestamp.UnixMilli()),
		Member: transaction.IdentificationCode,
	})

	_, err = pipeline.Exec(ctx)
	if err != nil {
		return fmt.Errorf("erro ao executar pipeline Redis: %w", err)
	}

	return nil
}

func (r *RedisMetricsRepository) RetrieveMetrics(processorType string, timeRange domain.TimeRange) (domain.MetricsData, error) {
	ctx := context.Background()
	result := domain.MetricsData{}

	transactionIDs, err := r.client.ZRangeByScore(ctx,
		fmt.Sprintf("metrics:%s:timeline", processorType),
		&redis.ZRangeBy{
			Min: fmt.Sprintf("%d", timeRange.StartTime.UnixMilli()),
			Max: fmt.Sprintf("%d", timeRange.EndTime.UnixMilli()),
		}).Result()

	if err != nil || len(transactionIDs) == 0 {
		return result, err
	}

	values, err := r.client.HMGet(ctx,
		fmt.Sprintf("metrics:%s:data", processorType),
		transactionIDs...).Result()

	if err != nil {
		return result, fmt.Errorf("erro ao recuperar dados do Redis: %w", err)
	}

	for _, value := range values {
		if valueStr, ok := value.(string); ok {
			if amount, err := strconv.ParseFloat(valueStr, 64); err == nil {
				result.TotalValue += amount
				result.TotalTransactions++
			}
		}
	}

	result.TotalValue = math.Round(result.TotalValue*100) / 100

	return result, nil
}

func (r *RedisMetricsRepository) ClearAllData() error {
	ctx := context.Background()
	return r.client.FlushAll(ctx).Err()
}
