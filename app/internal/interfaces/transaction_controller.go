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
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var transactionRequest domain.TransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&transactionRequest); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if !c.transactionHandler.IsQueueAvailable() {
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}

	if err := c.transactionHandler.SubmitTransaction(transactionRequest); err != nil {
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (c *TransactionController) HandleMetricsQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

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

	startTime, err1 := time.Parse(time.RFC3339, fromParam)
	endTime, err2 := time.Parse(time.RFC3339, toParam)

	if fromParam == "" || toParam == "" || err1 != nil || err2 != nil {
		return domain.TimeRange{
			StartTime: time.Unix(0, 0).UTC(),
			EndTime:   time.Now().UTC(),
		}
	}

	return domain.TimeRange{
		StartTime: startTime,
		EndTime:   endTime,
	}
}
