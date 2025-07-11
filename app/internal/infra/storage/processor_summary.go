package storage

import (
	"encoding/json"
	"net/http"
	"os"
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
	resp, err := http.Get(baseURL + "/admin/payments-summary")
	if err != nil {
		return ProcessorSummary{}, err
	}
	defer resp.Body.Close()
	var s ProcessorSummary
	json.NewDecoder(resp.Body).Decode(&s)
	return s, nil
}
