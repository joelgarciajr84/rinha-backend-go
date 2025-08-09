package infrastructure

import (
	"bytes"
	"encoding/json"
	"fmt"
	"galo/internal/domain"
	"net/http"
	"sync"
	"time"
)

type HTTPTransactionProcessor struct {
	httpClient         *http.Client
	primaryURL         string
	fallbackURL        string
	concurrencyLimiter chan struct{}
}

func NewHTTPTransactionProcessor(primaryURL, fallbackURL string, maxConcurrency int) *HTTPTransactionProcessor {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   false,
		DisableCompression:  true,
		ForceAttemptHTTP2:   false,
	}
	client := &http.Client{Timeout: 10 * time.Second, Transport: transport}
	return &HTTPTransactionProcessor{
		httpClient:         client,
		primaryURL:         primaryURL,
		fallbackURL:        fallbackURL,
		concurrencyLimiter: make(chan struct{}, maxConcurrency),
	}
}

func (p *HTTPTransactionProcessor) ExecuteTransaction(request domain.TransactionRequest, usePrimary bool) domain.ProcessingResult {
	p.concurrencyLimiter <- struct{}{}
	defer func() { <-p.concurrencyLimiter }()

	baseURL := p.primaryURL
	proc := "default"
	if !usePrimary {
		baseURL = p.fallbackURL
		proc = "fallback"
	}

	ok := p.sendTransactionRequest(request, baseURL)
	return domain.ProcessingResult{Success: ok, Processor: domain.ProcessorType(proc)}
}

func (p *HTTPTransactionProcessor) GetPrimaryURL() string {
	return p.primaryURL
}

func (p *HTTPTransactionProcessor) GetFallbackURL() string {
	return p.fallbackURL
}

var jsonBufferPool = sync.Pool{
	New: func() interface{} { return &bytes.Buffer{} },
}

func (p *HTTPTransactionProcessor) sendTransactionRequest(request domain.TransactionRequest, baseURL string) bool {
	buf := jsonBufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer jsonBufferPool.Put(buf)

	if err := json.NewEncoder(buf).Encode(request); err != nil {
		return false
	}

	endpoint := fmt.Sprintf("%s/payments", baseURL)

	httpRequest, err := http.NewRequest("POST", endpoint, buf)
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
