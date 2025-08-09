package interfaces

import (
	"encoding/json"
	"galo/internal/domain"
	"galo/internal/usecase"
	"net/http"
	"time"
)

type TransactionController struct {
	transactionHandler *usecase.TransactionHandler
	metricsAnalyzer    *usecase.MetricsAnalyzer
}

func NewTransactionController(
	transactionHandler *usecase.TransactionHandler,
	metricsAnalyzer *usecase.MetricsAnalyzer,
) *TransactionController {
	return &TransactionController{
		transactionHandler: transactionHandler,
		metricsAnalyzer:    metricsAnalyzer,
	}
}

func (c *TransactionController) HandleTransactionSubmission(w http.ResponseWriter, r *http.Request) {

	var transactionRequest domain.TransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&transactionRequest); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := c.transactionHandler.SubmitTransaction(transactionRequest); err != nil {
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (c *TransactionController) HandleMetricsQuery(w http.ResponseWriter, r *http.Request) {

	timeRange := c.parseTimeRangeFromQuery(r)
	metrics := c.metricsAnalyzer.GenerateTransactionReport(timeRange)

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(metrics); err != nil {
		http.Error(w, "Erro ao codificar resposta", http.StatusInternalServerError)
		return
	}
}

func (c *TransactionController) parseTimeRangeFromQuery(r *http.Request) domain.TimeRange {
	fromParam := r.URL.Query().Get("from")
	toParam := r.URL.Query().Get("to")

	parse := func(s string) (time.Time, bool) {
		if s == "" {
			return time.Time{}, false
		}
		// aceita qualquer precisão fracionária
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			return t, true
		}
		// fallback: RFC3339 “puro”
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t, true
		}
		return time.Time{}, false
	}

	var start, end time.Time
	if t, ok := parse(fromParam); ok {
		start = t
	}
	if t, ok := parse(toParam); ok {
		end = t
	}

	return domain.TimeRange{StartTime: start, EndTime: end}
}
