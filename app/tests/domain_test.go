package tests

import (
	"testing"
	"time"

	"galo/internal/domain"
)

func TestTransactionRequestCreation(t *testing.T) {
	transaction := domain.TransactionRequest{
		IdentificationCode: "test-123",
		MonetaryValue:      100.50,
		SubmittedAt:        time.Now().UTC().Format("2006-01-02T15:04:05.000Z07:00"),
	}

	if transaction.IdentificationCode != "test-123" {
		t.Errorf("Expected IdentificationCode to be 'test-123', got %s", transaction.IdentificationCode)
	}

	if transaction.MonetaryValue != 100.50 {
		t.Errorf("Expected MonetaryValue to be 100.50, got %f", transaction.MonetaryValue)
	}
}

func TestMetricsDataCalculation(t *testing.T) {
	metrics := domain.MetricsData{
		TotalTransactions: 5,
		TotalValue:        250.75,
	}

	if metrics.TotalTransactions != 5 {
		t.Errorf("Expected TotalTransactions to be 5, got %d", metrics.TotalTransactions)
	}

	if metrics.TotalValue != 250.75 {
		t.Errorf("Expected TotalValue to be 250.75, got %f", metrics.TotalValue)
	}
}

func TestTimeRangeCreation(t *testing.T) {
	now := time.Now()
	timeRange := domain.TimeRange{
		StartTime: now.Add(-1 * time.Hour),
		EndTime:   now,
	}

	if timeRange.EndTime.Before(timeRange.StartTime) {
		t.Error("EndTime should be after StartTime")
	}
}
