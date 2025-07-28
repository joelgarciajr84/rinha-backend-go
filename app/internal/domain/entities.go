package domain

import (
	"errors"
	"time"
)

var (
	ErrQueueFull = errors.New("fila de transações está cheia")
)

type TransactionRequest struct {
	IdentificationCode string    `json:"correlationId"`
	MonetaryValue      float64   `json:"amount"`
	SubmittedAt        string    `json:"requestedAt"`
	ProcessedAt        time.Time `json:"-"`
}

type MetricsData struct {
	TotalTransactions int64   `json:"totalRequests"`
	TotalValue        float64 `json:"totalAmount"`
}

type TransactionMetrics struct {
	PrimaryProcessor   MetricsData `json:"default"`
	SecondaryProcessor MetricsData `json:"fallback"`
}

type ProcessingResult struct {
	Success   bool
	Processor string
	Error     error
}

type TimeRange struct {
	StartTime time.Time
	EndTime   time.Time
}
