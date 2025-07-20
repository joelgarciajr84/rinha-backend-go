package adapter

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"rinha/model"

	"log/slog"

	"github.com/bytedance/sonic"
	"github.com/redis/go-redis/v9"
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
	defStatus := a.healthStatus.Default
	fallStatus := a.healthStatus.Fallback
	a.mu.RUnlock()

	if !defStatus.Failing && defStatus.MinResponseTime < 300 {
		if err := a.sendPayment(payment, a.defaultUrl+"/payments", 200*time.Millisecond); err == nil {
			return nil
		}
	}
	if !fallStatus.Failing && fallStatus.MinResponseTime < 100 {
		if err := a.sendPayment(payment, a.fallbackUrl+"/payments", 100*time.Millisecond); err == nil {
			return nil
		}
	}

	return model.ErrUnavailableProcessor
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

func (a *PaymentProcessorAdapter) Summary(from, to, token string) (model.SummaryResponse, error) {
	def, err := a.getSummary(a.defaultUrl+"/admin/payments-summary", from, to, token)
	if err != nil {
		return model.SummaryResponse{}, err
	}
	fallb, err := a.getSummary(a.fallbackUrl+"/admin/payments-summary", from, to, token)
	if err != nil {
		return model.SummaryResponse{}, err
	}
	return model.SummaryResponse{DefaultSummary: def, FallbackSummary: fallb}, nil
}

func (a *PaymentProcessorAdapter) getSummary(url, from, to, token string) (model.SummaryTotalRequestsResponse, error) {
	reqUrl := url + "?from=" + from + "&to=" + to
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, reqUrl, nil)
	req.Header.Set("X-Rinha-Token", token)

	res, err := a.client.Do(req)
	if err != nil {
		return model.SummaryTotalRequestsResponse{}, err
	}
	defer res.Body.Close()

	var out model.SummaryTotalRequestsResponse
	err = sonic.ConfigFastest.NewDecoder(res.Body).Decode(&out)
	return out, err
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
			for p := range a.retryQueue {
				time.Sleep(500 * time.Millisecond)
				a.Process(p)
			}
		}()
	}
}
