package infrastructure

import (
	"bytes"
	"encoding/json"
	"fmt"
	"galo/internal/domain"
	"net/http"
	"time"
)

type HTTPTransactionProcessor struct {
	httpClient         *http.Client
	primaryURL         string
	fallbackURL        string
	concurrencyLimiter chan struct{}
	currentURL         string
}

func NewHTTPTransactionProcessor(primaryURL, fallbackURL string, maxConcurrency int) *HTTPTransactionProcessor {
	return &HTTPTransactionProcessor{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		primaryURL:         primaryURL,
		fallbackURL:        fallbackURL,
		concurrencyLimiter: make(chan struct{}, maxConcurrency),
		currentURL:         primaryURL,
	}
}

func (p *HTTPTransactionProcessor) ExecuteTransaction(request domain.TransactionRequest) domain.ProcessingResult {

	p.concurrencyLimiter <- struct{}{}
	defer func() { <-p.concurrencyLimiter }()

	success := p.sendTransactionRequest(request, p.currentURL)

	return domain.ProcessingResult{
		Success:   success,
		Processor: p.determineProcessorType(p.currentURL),
	}
}

func (p *HTTPTransactionProcessor) SetProcessorURL(usePrimary bool) {
	if usePrimary {
		p.currentURL = p.primaryURL
	} else {
		p.currentURL = p.fallbackURL
	}
}

func (p *HTTPTransactionProcessor) GetPrimaryURL() string {
	return p.primaryURL
}

func (p *HTTPTransactionProcessor) GetFallbackURL() string {
	return p.fallbackURL
}

func (p *HTTPTransactionProcessor) sendTransactionRequest(request domain.TransactionRequest, baseURL string) bool {
	endpoint := fmt.Sprintf("%s/payments", baseURL)

	requestBody, err := json.Marshal(request)
	if err != nil {
		return false
	}

	httpRequest, err := http.NewRequest("POST", endpoint, bytes.NewReader(requestBody))
	if err != nil {
		return false
	}

	httpRequest.Header.Set("Content-Type", "application/json")

	response, err := p.httpClient.Do(httpRequest)
	if err != nil {
		return false
	}
	defer response.Body.Close()

	return response.StatusCode == http.StatusOK
}

func (p *HTTPTransactionProcessor) determineProcessorType(url string) string {
	if url == p.primaryURL {
		return "primary"
	}
	return "fallback"
}
