package domain

import (
	"errors"
	"time"
)

type TransactionRequest struct {
	CorrelationID string    `json:"correlationId"`
	Amount        float64   `json:"amount"`
	RequestedAt   time.Time `json:"requestedAt"`
}

func (t TransactionRequest) Validate() error {
	if t.CorrelationID == "" {
		return errors.New("invalid correlationId")
	}

	if t.Amount <= 0 {
		return errors.New("invalid amount")
	}

	return nil
}

type TransactionSummaryDTO struct {
	ProcessorType string
	Count         uint
	TotalAmount   float64
}

type TransactionSummaryResponse struct {
	Default  TransactionSummaryDetail `json:"default"`
	Fallback TransactionSummaryDetail `json:"fallback"`
}

type TransactionSummaryDetail struct {
	TotalRequests uint    `json:"totalRequests"`
	TotalAmount   float64 `json:"totalAmount"`
}

func BuildSummaryResponse(summaries []TransactionSummaryDTO) TransactionSummaryResponse {
	response := TransactionSummaryResponse{}

	for _, summary := range summaries {
		detail := TransactionSummaryDetail{
			TotalRequests: summary.Count,
			TotalAmount:   summary.TotalAmount,
		}

		if summary.ProcessorType == "primary" {
			response.Default = detail
		} else if summary.ProcessorType == "fallback" {
			response.Fallback = detail
		}
	}

	return response
}
