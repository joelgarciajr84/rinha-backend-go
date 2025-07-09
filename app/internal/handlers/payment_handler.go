package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"rinha/internal/core/domain"
	"rinha/internal/core/service"
)

type PaymentHandler struct {
	Service service.PaymentService
	Storage service.Storage
}

func NewHandler(s service.PaymentService, store service.Storage) *PaymentHandler {
	return &PaymentHandler{Service: s, Storage: store}
}

func (h *PaymentHandler) HandlePayment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var p domain.Payment
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if p.CorrelationID == "" || p.Amount <= 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if h.Storage.AlreadyProcessed(p.CorrelationID) {
		w.WriteHeader(http.StatusConflict)
		return
	}

	p.RequestedAt = time.Now().UTC().Format(time.RFC3339Nano)
	h.Storage.MarkProcessed(p.CorrelationID)
	h.Service.ProcessPayment(p)

	w.WriteHeader(http.StatusAccepted)
}

func (h *PaymentHandler) HandleSummary(w http.ResponseWriter, r *http.Request) {
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	var from, to *time.Time
	if fromStr != "" {
		t, err := time.Parse(time.RFC3339, fromStr)
		if err == nil {
			from = &t
		}
	}
	if toStr != "" {
		t, err := time.Parse(time.RFC3339, toStr)
		if err == nil {
			to = &t
		}
	}

	summary := h.Storage.GetSummary(from, to)
	fmt.Print("Summary: ", summary)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}
