package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"rinha-go-joel/src/domain"
	"rinha-go-joel/src/processor"
	"time"
)

type HTTPHandler struct {
	transactionProcessor *processor.TransactionProcessor
	transactionService   TransactionService
}

type TransactionService interface {
	GetTransactionSummary(ctx context.Context, from, to string) (domain.TransactionSummaryResponse, error)
}

func NewHTTPHandler(tp *processor.TransactionProcessor, ts TransactionService) HTTPHandler {
	return HTTPHandler{
		transactionProcessor: tp,
		transactionService:   ts,
	}
}

func (h HTTPHandler) HandleTransactions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req domain.TransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := req.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.transactionProcessor.SubmitTransaction(req); err != nil {
		http.Error(w, "Service temporarily unavailable", http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (h HTTPHandler) HandleTransactionSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")

	if from == "" {
		from = time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	}
	if to == "" {
		to = time.Now().Format(time.RFC3339)
	}

	summary, err := h.transactionService.GetTransactionSummary(r.Context(), from, to)
	if err != nil {
		http.Error(w, "Failed to get summary", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(summary); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (h HTTPHandler) HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	metrics := h.transactionProcessor.GetMetrics()

	health := map[string]interface{}{
		"status":           "healthy",
		"queue_size":       metrics.QueueSize,
		"avg_process_time": metrics.AvgProcessTime.String(),
		"processed":        metrics.Processed,
		"failed":           metrics.Failed,
		"retried":          metrics.Retried,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}
