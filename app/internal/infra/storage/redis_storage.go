package storage

import (
	"context"
	"strconv"
	"time"

	"rinha/internal/core/domain"

	"github.com/redis/go-redis/v9"
)

type RedisStorage struct {
	client *redis.Client
}

func NewRedisStorage(addr string) *RedisStorage {
	client := redis.NewClient(&redis.Options{
		Addr: addr,
		DB:   0,
	})
	return &RedisStorage{client: client}
}

func (r *RedisStorage) SavePayment(p domain.Payment, processor string) {
	ctx := context.Background()
	pipe := r.client.TxPipeline()
	pipe.RPush(ctx, "payments", marshalPayment(p, processor))
	pipe.Incr(ctx, "stats:"+processor+":totalRequests")
	pipe.IncrByFloat(ctx, "stats:"+processor+":totalAmount", p.Amount)
	pipe.Exec(ctx)
}

func (r *RedisStorage) AlreadyProcessed(id string) bool {
	ctx := context.Background()
	res, _ := r.client.Exists(ctx, "processed:"+id).Result()
	return res == 1
}

func (r *RedisStorage) MarkProcessed(id string) {
	ctx := context.Background()
	r.client.Set(ctx, "processed:"+id, 1, 0)
}

func (r *RedisStorage) UnmarkProcessing(id string) {
	ctx := context.Background()
	r.client.Del(ctx, "processed:"+id)
}

// TryMarkProcessing faz SETNX e retorna true se conseguiu marcar, false se já processado
func (r *RedisStorage) TryMarkProcessing(id string) bool {
	ctx := context.Background()
	res, _ := r.client.SetNX(ctx, "processed:"+id, 1, 0).Result()
	return res
}

func (r *RedisStorage) GetSummary(from, to *time.Time) domain.Summary {
	ctx := context.Background()
	defaultReq, _ := r.client.Get(ctx, "stats:default:totalRequests").Int()
	defaultAmt, _ := r.client.Get(ctx, "stats:default:totalAmount").Float64()
	fallbackReq, _ := r.client.Get(ctx, "stats:fallback:totalRequests").Int()
	fallbackAmt, _ := r.client.Get(ctx, "stats:fallback:totalAmount").Float64()
	return domain.Summary{
		Default:  domain.ProcessorStats{TotalRequests: defaultReq, TotalAmount: defaultAmt},
		Fallback: domain.ProcessorStats{TotalRequests: fallbackReq, TotalAmount: fallbackAmt},
	}
}

// Busca o summary real dos Payment Processors externos
func (r *RedisStorage) GetConsistentSummary(from, to *time.Time) domain.Summary {
	ext, err := GetProcessorSummary()
	if err != nil {
		// fallback para o summary local se falhar
		return r.GetSummary(from, to)
	}
	return domain.Summary{
		Default:  domain.ProcessorStats{TotalRequests: ext.Default.TotalRequests, TotalAmount: ext.Default.TotalAmount},
		Fallback: domain.ProcessorStats{TotalRequests: ext.Fallback.TotalRequests, TotalAmount: ext.Fallback.TotalAmount},
	}
}

// Helper para serializar pagamento (pode ser melhorado para JSON se necessário)
func marshalPayment(p domain.Payment, processor string) string {
	return p.CorrelationID + "," + strconv.FormatFloat(p.Amount, 'f', 2, 64) + "," + p.RequestedAt + "," + processor
}
