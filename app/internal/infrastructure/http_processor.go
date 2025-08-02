package infrastructure

import (
	"bytes"
	"encoding/json"
	"fmt"
	"galo/internal/domain"
	"net/http"
	"sync"
)

type HTTPTransactionProcessor struct {
	httpClient         *http.Client
	primaryURL         string
	fallbackURL        string
	concurrencyLimiter chan struct{}
	currentURL         string
}

func NewHTTPTransactionProcessor(primaryURL, fallbackURL string, maxConcurrency int) *HTTPTransactionProcessor {
	transport := &http.Transport{
		MaxIdleConns:        0,
		MaxIdleConnsPerHost: 0,
		IdleConnTimeout:     0,
		DisableCompression:  false,
		ForceAttemptHTTP2:   true,
	}

	return &HTTPTransactionProcessor{
		httpClient: &http.Client{
			Timeout:   0,
			Transport: transport,
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

func (p *HTTPTransactionProcessor) determineProcessorType(url string) string {
	if url == p.primaryURL {
		return "primary"
	}
	return "fallback"
}
