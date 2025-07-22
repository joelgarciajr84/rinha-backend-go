// 🔧 Arquivo: payment_processor.go
package adapter

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"rinha/model"

	"log/slog"

	"github.com/bytedance/sonic"
	"github.com/redis/go-redis/v9"
)

const (
	RedisKeyPrefixDefault  = "payments:default"
	RedisKeyPrefixFallback = "payments:fallback"
)

type PaymentProcessorAdapter struct {
	client       *http.Client
	db           *redis.Client
	healthStatus *model.HealthCheckStatus
	mu           sync.RWMutex
	defaultUrl   string
	fallbackUrl  string
	retryQueue   chan model.PaymentRequestProcessor
	workers      int
}

func NewPaymentProcessorAdapter(client *http.Client, db *redis.Client, defaultUrl, fallbackUrl string, retryQueue chan model.PaymentRequestProcessor, workers int) *PaymentProcessorAdapter {
	slog.Info("Creating PaymentProcessorAdapter", "defaultUrl", defaultUrl, "fallbackUrl", fallbackUrl)
	return &PaymentProcessorAdapter{
		client: client,
		db:     db,
		healthStatus: &model.HealthCheckStatus{
			Default:  model.HealthCheckResponse{},
			Fallback: model.HealthCheckResponse{},
		},
		defaultUrl:  defaultUrl,
		fallbackUrl: fallbackUrl,
		retryQueue:  retryQueue,
		workers:     workers,
	}
}

func (a *PaymentProcessorAdapter) Process(payment model.PaymentRequestProcessor) {
	ctx := context.Background()
	key := "correlation:" + payment.CorrelationId

	set, err := a.db.SetNX(ctx, key, "1", 1*time.Minute).Result()
	if err != nil {
		slog.Warn("Redis error", "err", err)
	} else if !set {
		slog.Debug("Duplicate correlationId, skipping", "correlationId", payment.CorrelationId)
		return
	}

	err = a.innerProcess(payment)
	if err != nil {
		if errors.Is(err, model.ErrInvalidRequest) {
			return
		}
		select {
		case a.retryQueue <- payment:
		default:
			slog.Warn("Retry queue full, discarding", "correlationId", payment.CorrelationId)
		}
	}
}

func (a *PaymentProcessorAdapter) innerProcess(payment model.PaymentRequestProcessor) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	ctx := context.Background()

	if !a.healthStatus.Default.Failing && a.healthStatus.Default.MinResponseTime < 300 {
		if err := a.sendPayment(payment, a.defaultUrl+"/payments", 200*time.Millisecond); err == nil {
			a.storePayment(ctx, RedisKeyPrefixDefault, payment)
			return nil
		}
	}
	if !a.healthStatus.Fallback.Failing && a.healthStatus.Fallback.MinResponseTime < 100 {
		if err := a.sendPayment(payment, a.fallbackUrl+"/payments", 100*time.Millisecond); err == nil {
			a.storePayment(ctx, RedisKeyPrefixFallback, payment)
			return nil
		}
	}
	return model.ErrUnavailableProcessor
}

func (a *PaymentProcessorAdapter) storePayment(ctx context.Context, keyPrefix string, payment model.PaymentRequestProcessor) {
	key := keyPrefix
	amountStr := strconv.FormatFloat(payment.Amount, 'f', 2, 64)
	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	member := amountStr + "|" + timestamp

	slog.Info("Storing payment", "key", key, "amount", amountStr, "timestamp", timestamp, "correlationId", payment.CorrelationId)

	score := float64(time.Now().UTC().UnixNano())
	res := a.db.ZAdd(ctx, key, redis.Z{Score: score, Member: member})
	if err := res.Err(); err != nil {
		slog.Error("Failed to store payment in Redis", "err", err, "correlationId", payment.CorrelationId)
	} else {
		slog.Info("Successfully stored payment in Redis", "key", key, "amount", amountStr, "correlationId", payment.CorrelationId)
	}
}

func (a *PaymentProcessorAdapter) sendPayment(payment model.PaymentRequestProcessor, url string, timeout time.Duration) error {
	payment.UpdateRequestTime()
	body, err := sonic.ConfigFastest.Marshal(payment)
	payment.Body = body
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := a.client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return model.ErrUnavailableProcessor
		}
		return err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case 422:
		return model.ErrInvalidRequest
	case 429, 500, 502, 503, 504, 408:
		return model.ErrUnavailableProcessor
	}
	return nil
}

func (a *PaymentProcessorAdapter) Summary(fromStr, toStr, _ string) (model.SummaryResponse, error) {
	ctx := context.Background()

	var from, to int64
	var err error
	if fromStr != "" {
		var fromTime time.Time
		fromTime, err = time.Parse(time.RFC3339Nano, fromStr)
		if err != nil {
			slog.Error("Invalid from timestamp", "fromStr", fromStr, "err", err)
			from = 0
		} else {
			from = fromTime.UnixNano()
		}
	}
	if toStr != "" {
		var toTime time.Time
		toTime, err = time.Parse(time.RFC3339Nano, toStr)
		if err != nil {
			slog.Error("Invalid to timestamp", "toStr", toStr, "err", err)
			to = time.Now().UTC().UnixNano()
		} else {
			to = toTime.UnixNano()
		}
	} else {
		to = time.Now().UTC().UnixNano()
	}

	getSummary := func(key string) model.SummaryTotalRequestsResponse {
		slog.Info("Summary query", "key", key, "from", from, "to", to)
		res := model.SummaryTotalRequestsResponse{}
		items, _ := a.db.ZRangeByScore(ctx, key, &redis.ZRangeBy{
			Min: strconv.FormatInt(from, 10),
			Max: strconv.FormatInt(to, 10),
		}).Result()
		for _, entry := range items {
			parts := strings.Split(entry, "|")
			if len(parts) != 2 {
				continue
			}
			val, _ := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
			res.TotalAmount += val
			res.TotalRequests++
		}
		return res
	}

	return model.SummaryResponse{
		DefaultSummary:  getSummary(RedisKeyPrefixDefault),
		FallbackSummary: getSummary(RedisKeyPrefixFallback),
	}, nil
}

func (a *PaymentProcessorAdapter) Purge(token string) error {
	for _, url := range []string{a.defaultUrl, a.fallbackUrl} {
		req, _ := http.NewRequest(http.MethodPost, url+"/admin/purge-payments", nil)
		req.Header.Set("X-Rinha-Token", token)
		res, err := a.client.Do(req)
		if err != nil || res.StatusCode != 200 {
			return model.ErrInvalidRequest
		}
		res.Body.Close()
	}
	return nil
}

func (a *PaymentProcessorAdapter) EnableHealthCheck(should string) {
	if should != "true" {
		return
	}

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			def, _ := a.healthCheck(a.defaultUrl + "/payments/service-health")
			fall, _ := a.healthCheck(a.fallbackUrl + "/payments/service-health")

			status := &model.HealthCheckStatus{Default: def, Fallback: fall}
			a.mu.Lock()
			a.healthStatus = status
			a.mu.Unlock()

			raw, _ := sonic.Marshal(status)
			a.db.Set(context.Background(), model.HealthCheckKey, raw, 30*time.Second)
		}
	}()
}

func (a *PaymentProcessorAdapter) healthCheck(url string) (model.HealthCheckResponse, error) {
	res, err := a.client.Get(url)
	if err != nil || res.StatusCode != 200 {
		return model.HealthCheckResponse{Failing: true, MinResponseTime: 9999}, err
	}
	defer res.Body.Close()

	var hc model.HealthCheckResponse
	_ = sonic.ConfigFastest.NewDecoder(res.Body).Decode(&hc)
	return hc, nil
}

func (a *PaymentProcessorAdapter) StartWorkers() {
	for i := 0; i < a.workers; i++ {
		go func() {
			time.Sleep(10 * time.Millisecond)
			for p := range a.retryQueue {
				a.Process(p)
			}
		}()
	}
}
