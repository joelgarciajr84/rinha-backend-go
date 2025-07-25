package gateway

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"rinha-go-joel/src/circuitbreaker"
	"rinha-go-joel/src/domain"
	"rinha-go-joel/src/healthmonitor"
	"rinha-go-joel/src/sentinel"
	"time"

	"github.com/goccy/go-json"
)

type ProcessorGateway struct {
	primaryClient   http.Client
	fallbackClient  http.Client
	primaryURL      string
	fallbackURL     string
	healthMonitor   *healthmonitor.HealthMonitor
	primaryBreaker  *circuitbreaker.CircuitBreaker
	fallbackBreaker *circuitbreaker.CircuitBreaker
	maxRetries      int
	baseRetryDelay  time.Duration
}

func NewProcessorGateway() (*ProcessorGateway, error) {
	primaryURL := getBaseURL("payment-processor-default:8080")
	fallbackURL := getBaseURL("payment-processor-fallback:8080")

	healthMon := &healthmonitor.HealthMonitor{}

	return &ProcessorGateway{
		primaryURL:      primaryURL,
		fallbackURL:     fallbackURL,
		primaryClient:   http.Client{Timeout: 200 * time.Millisecond},
		fallbackClient:  http.Client{Timeout: 250 * time.Millisecond},
		maxRetries:      0,
		baseRetryDelay:  25 * time.Millisecond,
		primaryBreaker:  circuitbreaker.NewCircuitBreaker(10, time.Second*60, 8),
		fallbackBreaker: circuitbreaker.NewCircuitBreaker(10, time.Second*60, 8),
		healthMonitor:   healthMon,
	}, nil
}

func (g ProcessorGateway) ProcessTransaction(ctx context.Context, req domain.TransactionRequest) (string, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	bestProcessor := g.healthMonitor.GetBestProcessor()

	if _, err := g.tryProcessor(ctx, bestProcessor, payload); err == nil {
		return string(bestProcessor), nil
	} else if sentinel.IsDismissible(err) {
		return "", err
	}

	alternativeProcessor := healthmonitor.Fallback
	if bestProcessor == healthmonitor.Fallback {
		alternativeProcessor = healthmonitor.Primary
	}

	if _, err := g.tryProcessor(ctx, alternativeProcessor, payload); err == nil {
		return string(alternativeProcessor), nil
	} else if sentinel.IsDismissible(err) {
		return "", err
	}

	return "", fmt.Errorf("both processors failed")
}

func (g ProcessorGateway) tryProcessor(ctx context.Context, processor healthmonitor.ProcessorType, payload []byte) (bool, error) {
	breaker := g.primaryBreaker
	client := g.primaryClient
	url := g.primaryURL

	if processor == healthmonitor.Fallback {
		breaker = g.fallbackBreaker
		client = g.fallbackClient
		url = g.fallbackURL
	}

	if !breaker.CanExecute() {
		return false, fmt.Errorf("circuit breaker is open for %s processor", processor)
	}

	for attempt := 0; attempt <= g.maxRetries; attempt++ {
		if attempt > 0 {
			delay := g.baseRetryDelay
			if delay > time.Millisecond*100 {
				delay = time.Millisecond * 100
			}
			time.Sleep(delay)
		}

		err := g.executeRequest(ctx, client, url, payload)
		if err == nil {
			breaker.OnSuccess()
			return true, nil
		}

		if sentinel.IsDismissible(err) {
			return false, err
		}

		if attempt >= g.maxRetries {
			break
		}
	}

	breaker.OnFailure()
	return false, fmt.Errorf("processor %s failed after %d attempts", processor, g.maxRetries+1)
}

func (g ProcessorGateway) executeRequest(ctx context.Context, client http.Client, url string, payload []byte) error {
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnprocessableEntity {
		return sentinel.NewGuardian(true, "transaction already processed")
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("processor returned status %d", resp.StatusCode)
	}

	return nil
}

func getBaseURL(host string) string {
	return fmt.Sprintf("http://%s/payments", host)
}
