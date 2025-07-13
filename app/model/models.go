package model

import (
	"errors"
	"time"
)

type PaymentRequest struct {
	CorrelationId string  `json:"correlationId"`
	Amount        float64 `json:"amount"`
}

type PaymentRequestProcessor struct {
	PaymentRequest
	RequestedAt *string `json:"requestedAt"`
	Body        []byte
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
