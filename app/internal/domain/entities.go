package domain

import (
	"errors"
	"time"
)

var (
	ErrQueueFull = errors.New("fila de transações está cheia")
)

// ProcessorType representa o destino do processamento.
// Use sempre DefaultProcessor / FallbackProcessor.
type ProcessorType string

const (
	DefaultProcessor  ProcessorType = "default"
	FallbackProcessor ProcessorType = "fallback"
)

type TransactionRequest struct {
	IdentificationCode string    `json:"correlationId"` // UUID vindo do cliente
	MonetaryValue      float64   `json:"amount"`        // valor em decimal (será convertido p/ centavos no repo)
	SubmittedAt        string    `json:"requestedAt"`   // RFC3339Nano (string para casar com a API do PP)
	ProcessedAt        time.Time `json:"-"`             // interno, não serializa
}

type MetricsData struct {
	TotalTransactions int64   `json:"totalRequests"`
	TotalValue        float64 `json:"totalAmount"`
}

type TransactionMetrics struct {
	PrimaryProcessor   MetricsData `json:"default"`
	SecondaryProcessor MetricsData `json:"fallback"`
}

// ProcessingResult agora usa o tipo forte; Success indica 2xx do PP.
type ProcessingResult struct {
	Success   bool
	Processor ProcessorType
	Error     error
}

type TimeRange struct {
	StartTime time.Time
	EndTime   time.Time
}
