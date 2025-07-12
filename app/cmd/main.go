package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

type PaymentRequest struct {
	CorrelationId string  `json:"correlationId"`
	Amount        float64 `json:"amount"`
}

type PaymentRequestProcessor struct {
	PaymentRequest
	RequestedAt *string `json:"requestedAt"`
}

func (p *PaymentRequestProcessor) UpdateRequestTime() {
	t := time.Now().UTC().Format(time.RFC3339Nano)
	p.RequestedAt = &t
}

type SummaryTotalRequestsResponse struct {
	TotalRequests int     `json:"totalRequests"`
	TotalAmount   float64 `json:"totalAmount"`
}

type SummaryResponse struct {
	DefaultSummary  SummaryTotalRequestsResponse `json:"default"`
	FallbackSummary SummaryTotalRequestsResponse `json:"fallback"`
}

type HealthCheckResponse struct {
	Failing         bool `json:"failing"`
	MinResponseTime int  `json:"minResponseTime"`
}

type HealthCheckStatus struct {
	Default  HealthCheckResponse `json:"default"`
	Fallback HealthCheckResponse `json:"fallback"`
}

var (
	ErrInvalidRequest       = errors.New("invalid request")
	ErrUnavailableProcessor = errors.New("unavailable processor")
	HealthCheckKey          = "health-check"
)

type PaymentProcessorAdapter struct {
	client       *http.Client
	db           *redis.Client
	healthStatus *HealthCheckStatus
	mu           sync.RWMutex
	defaultUrl   string
	fallbackUrl  string
	retryQueue   chan PaymentRequestProcessor
	workers      int
}

func NewPaymentProcessorAdapter(client *http.Client, db *redis.Client, defaultUrl, fallbackUrl string, retryQueue chan PaymentRequestProcessor, workers int) *PaymentProcessorAdapter {
	return &PaymentProcessorAdapter{
		client: client,
		db:     db,
		healthStatus: &HealthCheckStatus{
			Default:  HealthCheckResponse{},
			Fallback: HealthCheckResponse{},
		},
		defaultUrl:  defaultUrl,
		fallbackUrl: fallbackUrl,
		retryQueue:  retryQueue,
		workers:     workers,
	}
}

func (a *PaymentProcessorAdapter) Process(payment PaymentRequestProcessor) {
	ctx := context.Background()
	key := "correlation:" + payment.CorrelationId

	// Garantir idempotência
	set, err := a.db.SetNX(ctx, key, "1", 1*time.Minute).Result()
	if err != nil {
		slog.Warn("Redis error", "err", err)
	} else if !set {
		slog.Debug("Duplicate correlationId, skipping", "correlationId", payment.CorrelationId)
		return
	}

	err = a.innerProcess(payment)
	if err != nil {
		// Não retry se inválido
		if errors.Is(err, ErrInvalidRequest) {
			return
		}
		select {
		case a.retryQueue <- payment:
		default:
			slog.Warn("Retry queue full, discarding", "correlationId", payment.CorrelationId)
		}
	}
}

func (a *PaymentProcessorAdapter) innerProcess(payment PaymentRequestProcessor) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if !a.healthStatus.Default.Failing && a.healthStatus.Default.MinResponseTime < 300 {
		if err := a.sendPayment(payment, a.defaultUrl+"/payments", 200*time.Millisecond); err == nil {
			return nil
		}
	}
	if !a.healthStatus.Fallback.Failing && a.healthStatus.Fallback.MinResponseTime < 100 {
		if err := a.sendPayment(payment, a.fallbackUrl+"/payments", 100*time.Millisecond); err == nil {
			return nil
		}
	}

	return ErrUnavailableProcessor
}

func (a *PaymentProcessorAdapter) sendPayment(payment PaymentRequestProcessor, url string, timeout time.Duration) error {
	payment.UpdateRequestTime()

	body, err := sonic.ConfigFastest.Marshal(payment)
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
			return ErrUnavailableProcessor
		}
		return err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case 422:
		return ErrInvalidRequest
	case 429, 500, 502, 503, 504, 408:
		return ErrUnavailableProcessor
	}

	return nil
}

func (a *PaymentProcessorAdapter) Summary(from, to, token string) (SummaryResponse, error) {
	def, err := a.getSummary(a.defaultUrl+"/admin/payments-summary", from, to, token)
	if err != nil {
		return SummaryResponse{}, err
	}
	fallb, err := a.getSummary(a.fallbackUrl+"/admin/payments-summary", from, to, token)
	if err != nil {
		return SummaryResponse{}, err
	}
	return SummaryResponse{DefaultSummary: def, FallbackSummary: fallb}, nil
}

func (a *PaymentProcessorAdapter) getSummary(url, from, to, token string) (SummaryTotalRequestsResponse, error) {
	reqUrl := url + "?from=" + from + "&to=" + to
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, reqUrl, nil)
	req.Header.Set("X-Rinha-Token", token)

	res, err := a.client.Do(req)
	if err != nil {
		return SummaryTotalRequestsResponse{}, err
	}
	defer res.Body.Close()

	var out SummaryTotalRequestsResponse
	err = sonic.ConfigFastest.NewDecoder(res.Body).Decode(&out)
	return out, err
}

func (a *PaymentProcessorAdapter) Purge(token string) error {
	for _, url := range []string{a.defaultUrl, a.fallbackUrl} {
		req, _ := http.NewRequest(http.MethodPost, url+"/admin/purge-payments", nil)
		req.Header.Set("X-Rinha-Token", token)
		res, err := a.client.Do(req)
		if err != nil || res.StatusCode != 200 {
			return ErrInvalidRequest
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

			status := &HealthCheckStatus{Default: def, Fallback: fall}
			a.mu.Lock()
			a.healthStatus = status
			a.mu.Unlock()

			raw, _ := sonic.Marshal(status)
			a.db.Set(context.Background(), HealthCheckKey, raw, 30*time.Second)
		}
	}()
}

func (a *PaymentProcessorAdapter) healthCheck(url string) (HealthCheckResponse, error) {
	res, err := a.client.Get(url)
	if err != nil || res.StatusCode != 200 {
		return HealthCheckResponse{Failing: true, MinResponseTime: 9999}, err
	}
	defer res.Body.Close()

	var hc HealthCheckResponse
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

type PaymentHandler struct {
	adapter *PaymentProcessorAdapter
}

func NewPaymentHandler(a *PaymentProcessorAdapter) *PaymentHandler {
	return &PaymentHandler{adapter: a}
}

func (h *PaymentHandler) Process(c *fiber.Ctx) error {
	var req PaymentRequest
	if err := c.BodyParser(&req); err != nil || req.CorrelationId == "" || req.Amount <= 0 {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	go h.adapter.Process(PaymentRequestProcessor{PaymentRequest: req})
	return c.SendStatus(fiber.StatusAccepted)
}

func (h *PaymentHandler) Summary(c *fiber.Ctx) error {
	summary, err := h.adapter.Summary(c.Query("from"), c.Query("to"), c.Get("X-Rinha-Token", "123"))
	if err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	return c.JSON(summary)
}

func (h *PaymentHandler) Purge(c *fiber.Ctx) error {
	if err := h.adapter.Purge(c.Get("X-Rinha-Token", "123")); err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	return c.SendStatus(fiber.StatusOK)
}

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	slog.SetLogLoggerLevel(slog.LevelInfo)

	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        30,
			MaxIdleConnsPerHost: 15,
			IdleConnTimeout:     120 * time.Second,
			MaxConnsPerHost:     20,
			DialContext: (&net.Dialer{
				Timeout:   time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
		},
		Timeout: 5 * time.Second,
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: getEnvOrDefault("REDIS_ADDR", "localhost:6379"),
	})

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		slog.Error("Redis failed", "err", err)
		os.Exit(1)
	}

	retryQueue := make(chan PaymentRequestProcessor, 5000)
	adapter := NewPaymentProcessorAdapter(
		client,
		rdb,
		getEnvOrDefault("PAYMENT_PROCESSOR_URL_DEFAULT", "http://localhost:8001"),
		getEnvOrDefault("PAYMENT_PROCESSOR_URL_FALLBACK", "http://localhost:8002"),
		retryQueue,
		500,
	)

	handler := NewPaymentHandler(adapter)

	app := fiber.New(fiber.Config{
		JSONEncoder: sonic.Marshal,
		JSONDecoder: sonic.Unmarshal,
	})

	app.Post("/payments", handler.Process)
	app.Get("/payments-summary", handler.Summary)
	app.Post("/purge-payments", handler.Purge)

	adapter.StartWorkers()
	adapter.EnableHealthCheck(getEnvOrDefault("MONITOR_HEALTH", "true"))

	slog.Info("server running", "port", getEnvOrDefault("PORT", "9999"))
	if err := app.Listen(":" + getEnvOrDefault("PORT", "9999")); err != nil {
		slog.Error("server failed", "err", err)
	}
}
