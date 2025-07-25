package healthmonitor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type (
	HealthMonitor struct {
		primaryURL       string
		fallbackURL      string
		primaryClient    http.Client
		fallbackClient   http.Client
		cache            sync.Map
		lastCheck        sync.Map
		primaryInterval  time.Duration
		fallbackInterval time.Duration
	}

	ServiceHealth struct {
		Failing         bool `json:"failing"`
		MinResponseTime int  `json:"minResponseTime"`
		LastChecked     time.Time
		Available       bool
	}

	ProcessorType string
)

const (
	Primary  ProcessorType = "primary"
	Fallback ProcessorType = "fallback"
)

func NewHealthMonitor(primaryURL, fallbackURL string) *HealthMonitor {
	return &HealthMonitor{
		primaryURL:       fmt.Sprintf("%s/payments/service-health", primaryURL),
		fallbackURL:      fmt.Sprintf("%s/payments/service-health", fallbackURL),
		primaryClient:    http.Client{Timeout: time.Millisecond * 450},
		fallbackClient:   http.Client{Timeout: time.Millisecond * 450},
		primaryInterval:  time.Second * 6,
		fallbackInterval: time.Second * 6,
	}
}

func (h *HealthMonitor) GetHealthStatus(processor ProcessorType) (*ServiceHealth, error) {
	checkInterval := h.primaryInterval
	client := h.primaryClient
	url := h.primaryURL
	timeoutMs := 400

	if processor == Fallback {
		checkInterval = h.fallbackInterval
		client = h.fallbackClient
		url = h.fallbackURL
		timeoutMs = 400
	}

	if cached, ok := h.cache.Load(processor); ok {
		health := cached.(*ServiceHealth)
		if time.Since(health.LastChecked) < checkInterval {
			return health, nil
		}
	}

	if lastCheck, ok := h.lastCheck.Load(processor); ok {
		if time.Since(lastCheck.(time.Time)) < checkInterval {
			if cached, ok := h.cache.Load(processor); ok {
				return cached.(*ServiceHealth), nil
			}
			return &ServiceHealth{
				Failing:         true,
				MinResponseTime: 1000,
				LastChecked:     time.Now(),
				Available:       false,
			}, nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*time.Duration(timeoutMs))
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return h.handleHealthCheckError(processor), nil
	}

	resp, err := client.Do(req)
	if err != nil {
		return h.handleHealthCheckError(processor), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		if cached, ok := h.cache.Load(processor); ok {
			return cached.(*ServiceHealth), nil
		}
		return h.handleHealthCheckError(processor), nil
	}

	if resp.StatusCode != http.StatusOK {
		return h.handleHealthCheckError(processor), nil
	}

	var health ServiceHealth
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return h.handleHealthCheckError(processor), nil
	}

	health.LastChecked = time.Now()
	health.Available = !health.Failing
	h.cache.Store(processor, &health)
	h.lastCheck.Store(processor, time.Now())

	return &health, nil
}

func (h *HealthMonitor) handleHealthCheckError(processor ProcessorType) *ServiceHealth {
	health := &ServiceHealth{
		Failing:         true,
		MinResponseTime: 2000,
		LastChecked:     time.Now(),
		Available:       false,
	}

	h.cache.Store(processor, health)
	h.lastCheck.Store(processor, time.Now())

	return health
}

func (h *HealthMonitor) GetBestProcessor() ProcessorType {
	primaryHealth, _ := h.GetHealthStatus(Primary)
	fallbackHealth, _ := h.GetHealthStatus(Fallback)



	if primaryHealth.Available {
		return Primary
	}

	if fallbackHealth.Available {
		return Fallback
	}

	return Primary
}

func (h *HealthMonitor) IsHealthy(processor ProcessorType) bool {
	health, _ := h.GetHealthStatus(processor)
	return health.Available
}

func (h *HealthMonitor) GetExpectedLatency(processor ProcessorType) int {
	health, _ := h.GetHealthStatus(processor)
	return health.MinResponseTime
}
