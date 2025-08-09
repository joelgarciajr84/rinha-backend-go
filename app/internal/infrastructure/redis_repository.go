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
		// >>> tunings
		PoolSize:        80, // antes 10
		MinIdleConns:    8,
		DialTimeout:     100 * time.Millisecond,
		ReadTimeout:     150 * time.Millisecond,
		WriteTimeout:    150 * time.Millisecond,
		PoolTimeout:     60 * time.Second,
		ConnMaxIdleTime: 30 * time.Second,
		MaxRetries:      2, // menos backoff
		MinRetryBackoff: 1, // agressivo
		MaxRetryBackoff: 1 * time.Millisecond,
	})
	return &RedisMetricsRepository{client: client}
}

// func NewRedisMetricsRepository(redisURL string) *RedisMetricsRepository {
// 	client := redis.NewClient(&redis.Options{
// 		Addr:         redisURL,
// 		Password:     "",
// 		DB:           0,
// 		PoolSize:     64,
// 		MinIdleConns: 1,
// 		PoolTimeout:  60 * time.Second,
// 		MaxRetries:   2,
// 	})

// 	return &RedisMetricsRepository{
// 		client: client,
// 	}
// }

var ctx = context.Background()

func (r *RedisMetricsRepository) StoreTransactionData(processorType string, tx domain.TransactionRequest) error {
	ctx := context.Background()

	ts, err := time.Parse(time.RFC3339Nano, tx.SubmittedAt)
	if err != nil {
		return fmt.Errorf("erro ao converter timestamp: %w", err)
	}

	// valor em centavos
	cents := int64(math.Round(tx.MonetaryValue * 100.0))
	centsStr := strconv.FormatInt(cents, 10)

	dataKey := fmt.Sprintf("metrics:%s:data", processorType)
	timelineKey := fmt.Sprintf("metrics:%s:timeline", processorType)
	secKey := fmt.Sprintf("sum:%s:%d", processorType, ts.Unix()) // bucket por segundo
	txKey := fmt.Sprintf("tx:%s", tx.IdentificationCode)         // guard idempotente
	score := strconv.FormatInt(ts.UnixMilli(), 10)

	// TTLs (ajuste se quiser): bucket 2h, guarda idempotência 24h
	const bucketTTLSeconds = 2 * 60 * 60
	const txTTLms = 24 * 60 * 60 * 1000

	const lua = `
		-- KEYS[1]=dataKey, KEYS[2]=timelineKey, KEYS[3]=secKey, KEYS[4]=txKey
		-- ARGV[1]=id, ARGV[2]=centsStr, ARGV[3]=score, ARGV[4]=bucketTTL, ARGV[5]=txTTLms

		-- Guard idempotente: se já vimos esse id, sai sem somar de novo
		if redis.call('SETNX', KEYS[4], '1') == 0 then
			return 0
		end
		redis.call('PEXPIRE', KEYS[4], ARGV[5])

		-- Persistência por id (compat), timeline e bucket por segundo (contagem + soma)
		redis.call('HSET', KEYS[1], ARGV[1], ARGV[2])
		redis.call('ZADD', KEYS[2], ARGV[3], ARGV[1])
		redis.call('HINCRBY', KEYS[3], 'count', 1)
		redis.call('HINCRBY', KEYS[3], 'amountCents', tonumber(ARGV[2]))
		redis.call('EXPIRE', KEYS[3], tonumber(ARGV[4]))
		return 1
	`

	if _, err := r.client.Eval(ctx, lua,
		[]string{dataKey, timelineKey, secKey, txKey},
		tx.IdentificationCode, centsStr, score, strconv.Itoa(bucketTTLSeconds), strconv.Itoa(txTTLms),
	).Int(); err != nil {
		return fmt.Errorf("erro ao gravar métricas (Lua idempotente): %w", err)
	}
	return nil
}

func (r *RedisMetricsRepository) RetrieveMetrics(processorType string, tr domain.TimeRange) (domain.MetricsData, error) {
	ctx := context.Background()
	out := domain.MetricsData{}

	timelineKey := fmt.Sprintf("metrics:%s:timeline", processorType)
	dataKey := fmt.Sprintf("metrics:%s:data", processorType)

	// Monta bounds inclusivos (casam com o PP)
	min, max := "-inf", "+inf"
	if !tr.StartTime.IsZero() {
		min = fmt.Sprintf("%d", tr.StartTime.UnixMilli())
	}
	if !tr.EndTime.IsZero() {
		max = fmt.Sprintf("%d", tr.EndTime.UnixMilli())
	}

	// 1) CONTAGEM À PROVA DE CORRIDA (um comando no servidor)
	cnt, err := r.client.ZCount(ctx, timelineKey, min, max).Result()
	if err != nil {
		return out, fmt.Errorf("erro ZCOUNT: %w", err)
	}
	out.TotalTransactions = cnt

	// 2) SOMA (não impacta inconsistência; pode manter buckets/antigo)
	//    Sugestão: híbrido (bordas exatas + miolo buckets). Se quiser minimalista, keep como está.
	if tr.StartTime.IsZero() || tr.EndTime.IsZero() {
		// sem janela explícita → usa caminho antigo por compat
		return r.retrieveByTimelineFallback(processorType, tr)
	}

	// --- HÍBRIDO: bordas por ms + miolo por buckets ---
	startMs := tr.StartTime.UnixMilli()
	endMs := tr.EndTime.UnixMilli()
	startSec := tr.StartTime.Unix()
	endSec := tr.EndTime.Unix()

	// mesma função de antes; mantemos seu somatório em centavos
	if startSec == endSec {
		part, err := r.retrieveByTimelineExactRange(ctx, processorType, dataKey, timelineKey, startMs, endMs)
		if err != nil {
			return out, err
		}
		out.TotalValue += part.TotalValue
		return out, nil
	}

	// parcial inicial
	firstEndMs := startSec*1000 + 999
	partA, err := r.retrieveByTimelineExactRange(ctx, processorType, dataKey, timelineKey, startMs, firstEndMs)
	if err != nil {
		return out, err
	}
	out.TotalValue += partA.TotalValue

	// miolo por buckets
	midStartSec := startSec + 1
	midEndSec := endSec - 1
	if midEndSec >= midStartSec {
		if err := r.sumBucketsRange(ctx, processorType, midStartSec, midEndSec, &out); err != nil {
			return out, err
		}
	}

	// parcial final
	lastStartMs := endSec * 1000
	partC, err := r.retrieveByTimelineExactRange(ctx, processorType, dataKey, timelineKey, lastStartMs, endMs)
	if err != nil {
		return out, err
	}
	out.TotalValue += partC.TotalValue

	return out, nil
}

func (r *RedisMetricsRepository) sumBucketsRange(ctx context.Context, processorType string, fromSec, toSec int64, out *domain.MetricsData) error {
	const batch = 300
	keys := make([]string, 0, batch)

	flush := func() error {
		if len(keys) == 0 {
			return nil
		}
		pipe := r.client.Pipeline()
		cmds := make([]*redis.SliceCmd, 0, len(keys))
		for _, k := range keys {
			// pedimos só o amount; a contagem (count) NÃO deve ser acumulada aqui
			cmds = append(cmds, pipe.HMGet(ctx, k, "amountCents"))
		}
		if _, err := pipe.Exec(ctx); err != nil {
			return fmt.Errorf("erro pipeline buckets: %w", err)
		}
		for _, c := range cmds {
			vals, err := c.Result()
			if err != nil || len(vals) == 0 {
				continue
			}
			// vals[0] = amountCents
			if vals[0] != nil {
				if s, ok := vals[0].(string); ok && s != "" {
					if cents, err := strconv.ParseInt(s, 10, 64); err == nil {
						out.TotalValue += float64(cents) / 100.0
					}
				}
			}
		}
		keys = keys[:0]
		return nil
	}

	for sec := fromSec; sec <= toSec; sec++ {
		keys = append(keys, fmt.Sprintf("sum:%s:%d", processorType, sec))
		if len(keys) == batch {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	return flush()
}

// Leitura exata no intervalo de milissegundos [minMs, maxMs] (ambos inclusivos)
func (r *RedisMetricsRepository) retrieveByTimelineExactRange(
	ctx context.Context,
	processorType, dataKey, timelineKey string,
	minMs, maxMs int64,
) (domain.MetricsData, error) {
	res := domain.MetricsData{}
	min := fmt.Sprintf("%d", minMs)
	max := fmt.Sprintf("%d", maxMs)

	ids, err := r.client.ZRangeByScore(ctx, timelineKey, &redis.ZRangeBy{Min: min, Max: max}).Result()
	if err != nil || len(ids) == 0 {
		return res, err
	}

	// count é o número de IDs retornados (ground-truth do período)
	res.TotalTransactions = int64(len(ids))

	vals, err := r.client.HMGet(ctx, dataKey, ids...).Result()
	if err != nil {
		return res, fmt.Errorf("erro ao recuperar dados do Redis: %w", err)
	}

	var centsTotal int64
	// Retry leve para valores que vierem nil (janela mínima)
	missingIdx := make([]int, 0, 8)
	for i, v := range vals {
		if v == nil {
			missingIdx = append(missingIdx, i)
		}
	}
	if len(missingIdx) > 0 {
		fields := make([]string, 0, len(missingIdx))
		for _, i := range missingIdx {
			fields = append(fields, ids[i])
		}
		if retry, err2 := r.client.HMGet(ctx, dataKey, fields...).Result(); err2 == nil {
			for j, i := range missingIdx {
				vals[i] = retry[j]
			}
		}
	}

	for _, v := range vals {
		if v == nil {
			continue
		}
		if s, ok := v.(string); ok && s != "" {
			if cents, err := strconv.ParseInt(s, 10, 64); err == nil {
				centsTotal += cents
				continue
			}
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				centsTotal += int64(math.Round(f * 100.0))
			}
		}
	}
	res.TotalValue = float64(centsTotal) / 100.0
	return res, nil
}

// Caminho antigo, usado só como fallback se faltar from/to:
func (r *RedisMetricsRepository) retrieveByTimelineFallback(processorType string, tr domain.TimeRange) (domain.MetricsData, error) {
	ctx := context.Background()
	res := domain.MetricsData{}

	min, max := "-inf", "+inf"
	if !tr.StartTime.IsZero() {
		min = fmt.Sprintf("%d", tr.StartTime.UnixMilli())
	}
	if !tr.EndTime.IsZero() {
		max = fmt.Sprintf("%d", tr.EndTime.UnixMilli())
	}

	timelineKey := fmt.Sprintf("metrics:%s:timeline", processorType)
	dataKey := fmt.Sprintf("metrics:%s:data", processorType)

	ids, err := r.client.ZRangeByScore(ctx, timelineKey, &redis.ZRangeBy{Min: min, Max: max}).Result()
	if err != nil || len(ids) == 0 {
		return res, err
	}

	vals, err := r.client.HMGet(ctx, dataKey, ids...).Result()
	if err != nil {
		return res, fmt.Errorf("erro ao recuperar dados do Redis: %w", err)
	}

	var centsTotal int64
	for _, v := range vals {
		if v == nil {
			continue
		}
		if s, ok := v.(string); ok && s != "" {
			if cents, err := strconv.ParseInt(s, 10, 64); err == nil {
				centsTotal += cents
				continue
			}
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				centsTotal += int64(math.Round(f * 100.0))
			}
		}
	}
	res.TotalTransactions = int64(len(ids))
	res.TotalValue = float64(centsTotal) / 100.0
	return res, nil
}

func (r *RedisMetricsRepository) ClearAllData() error {
	ctx := context.Background()
	delByPattern := func(pattern string) error {
		iter := r.client.Scan(ctx, 0, pattern, 1000).Iterator()
		for iter.Next(ctx) {
			if err := r.client.Del(ctx, iter.Val()).Err(); err != nil {
				return err
			}
		}
		return iter.Err()
	}
	if err := delByPattern("metrics:*"); err != nil {
		return err
	}
	if err := delByPattern("sum:*"); err != nil {
		return err
	}
	if err := delByPattern("tx:*"); err != nil {
		return err
	}
	return nil
}
