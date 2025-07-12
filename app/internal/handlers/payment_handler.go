package handlers

import (
	"encoding/json"
	"net/http"

	"rinha/internal/core/domain"
	"rinha/internal/core/service"
)

type PaymentHandler struct {
	Service service.PaymentService
	Storage service.Storage
}

func NewHandler(s service.PaymentService, store service.Storage) *PaymentHandler {
	return &PaymentHandler{
		Service: s,
		Storage: store,
	}
}

func (h *PaymentHandler) HandlePayment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
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

	if err := h.Service.ProcessPayment(p); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (h *PaymentHandler) HandleSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	summary := h.Storage.GetSummary(nil, nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}
