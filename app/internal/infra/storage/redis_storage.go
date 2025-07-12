package storage

import (
	"context"
	"fmt"
	"log"
	"time"

	"rinha/internal/core/domain"

	"github.com/redis/go-redis/v9"
)

type RedisStorage struct {
	client *redis.Client
	ctx    context.Context
}

func NewRedisStorage(addr string) *RedisStorage {
	client := redis.NewClient(&redis.Options{
		Addr:            addr,
		DB:              0,
		PoolSize:        100,
		MinIdleConns:    20,
		MaxIdleConns:    50,
		ConnMaxIdleTime: 5 * time.Minute,
		DialTimeout:     500 * time.Millisecond,
		ReadTimeout:     300 * time.Millisecond,
		WriteTimeout:    300 * time.Millisecond,
		MaxRetries:      2,
		MinRetryBackoff: 8 * time.Millisecond,
		MaxRetryBackoff: 50 * time.Millisecond,
	})

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("Aviso: Redis não disponível: %v", err)
	}

	return &RedisStorage{
		client: client,
		ctx:    ctx,
	}
}

func (r *RedisStorage) SavePayment(p domain.Payment, processor string) {
	ctx, cancel := context.WithTimeout(r.ctx, 500*time.Millisecond)
	defer cancel()

	txf := func(tx *redis.Tx) error {
		exists, err := tx.Exists(ctx, "processed:"+p.CorrelationID).Result()
		if err != nil {
			return err
		}
		if exists == 1 {
			return fmt.Errorf("already processed")
		}

		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Incr(ctx, "stats:"+processor+":totalRequests")
			pipe.IncrByFloat(ctx, "stats:"+processor+":totalAmount", p.Amount)
			pipe.Set(ctx, "processed:"+p.CorrelationID, 1, 30*time.Minute)
			return nil
		})
		return err
	}

	for i := 0; i < 3; i++ {
		err := r.client.Watch(ctx, txf, "processed:"+p.CorrelationID)
		if err == nil {
			return
		}
		if err == redis.TxFailedErr {
			continue
		}
		log.Printf("Erro salvando pagamento %s: %v", p.CorrelationID, err)
		return
	}

	log.Printf("Falha ao salvar pagamento %s após 3 tentativas", p.CorrelationID)
}

func (r *RedisStorage) AlreadyProcessed(id string) bool {
	ctx, cancel := context.WithTimeout(r.ctx, 100*time.Millisecond)
	defer cancel()

	exists, err := r.client.Exists(ctx, "processed:"+id).Result()
	if err != nil {
		return false
	}
	return exists == 1
}

func (r *RedisStorage) GetSummary(from, to *time.Time) domain.Summary {
	ctx, cancel := context.WithTimeout(r.ctx, 200*time.Millisecond)
	defer cancel()

	pipe := r.client.Pipeline()

	defaultReqCmd := pipe.Get(ctx, "stats:default:totalRequests")
	defaultAmtCmd := pipe.Get(ctx, "stats:default:totalAmount")
	fallbackReqCmd := pipe.Get(ctx, "stats:fallback:totalRequests")
	fallbackAmtCmd := pipe.Get(ctx, "stats:fallback:totalAmount")

	_, err := pipe.Exec(ctx)
	if err != nil {
		log.Printf("Erro buscando summary: %v", err)
		return domain.Summary{}
	}

	defaultReq, _ := defaultReqCmd.Int()
	defaultAmt, _ := defaultAmtCmd.Float64()
	fallbackReq, _ := fallbackReqCmd.Int()
	fallbackAmt, _ := fallbackAmtCmd.Float64()

	return domain.Summary{
		Default: domain.ProcessorStats{
			TotalRequests: defaultReq,
			TotalAmount:   defaultAmt,
		},
		Fallback: domain.ProcessorStats{
			TotalRequests: fallbackReq,
			TotalAmount:   fallbackAmt,
		},
	}
}
