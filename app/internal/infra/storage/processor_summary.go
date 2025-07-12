package storage

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"
)

type ProcessorSummary struct {
	TotalRequests int     `json:"totalRequests"`
	TotalAmount   float64 `json:"totalAmount"`
}

type ExternalSummary struct {
	Default  ProcessorSummary `json:"default"`
	Fallback ProcessorSummary `json:"fallback"`
}

func GetProcessorSummary() (ExternalSummary, error) {
	urlDefault := os.Getenv("PAYMENT_PROCESSOR_URL_DEFAULT")
	urlFallback := os.Getenv("PAYMENT_PROCESSOR_URL_FALLBACK")

	if urlDefault == "" || urlFallback == "" {
		return ExternalSummary{}, errors.New("missing payment processor URLs in environment variables")
	}

	defaultSummary, err := fetchSummary(urlDefault)
	if err != nil {
		return ExternalSummary{}, err
	}

	fallbackSummary, err := fetchSummary(urlFallback)
	if err != nil {
		return ExternalSummary{}, err
	}

	return ExternalSummary{
		Default:  defaultSummary,
		Fallback: fallbackSummary,
	}, nil
}

func fetchSummary(baseURL string) (ProcessorSummary, error) {
	client := &http.Client{
		Timeout: 1 * time.Second,
	}

	resp, err := client.Get(baseURL + "/admin/payments-summary")
	if err != nil {
		return ProcessorSummary{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ProcessorSummary{}, errors.New("non-200 response from payment processor summary endpoint")
	}

	var s ProcessorSummary
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return ProcessorSummary{}, err
	}

	return s, nil
}
