package usecase

import (
	"galo/internal/domain"
	"log"
	"time"
)

type TransactionHandler struct {
	primaryProcessorURL  string
	fallbackProcessorURL string
	metricsRepo          domain.MetricsRepository
	transactionProcessor domain.ConfigurableProcessor
	queueManager         domain.QueueManager
}

func NewTransactionHandler(
	primaryURL, fallbackURL string,
	metricsRepo domain.MetricsRepository,
	processor domain.ConfigurableProcessor,
	queueManager domain.QueueManager,
) *TransactionHandler {
	return &TransactionHandler{
		primaryProcessorURL:  primaryURL,
		fallbackProcessorURL: fallbackURL,
		metricsRepo:          metricsRepo,
		transactionProcessor: processor,
		queueManager:         queueManager,
	}
}

func (h *TransactionHandler) SubmitTransaction(request domain.TransactionRequest) error {
	return h.queueManager.EnqueueTransaction(request)
}

func (h *TransactionHandler) ProcessTransaction(request domain.TransactionRequest) {
	request.SubmittedAt = time.Now().UTC().Format("2006-01-02T15:04:05.000Z07:00")

	h.transactionProcessor.SetProcessorURL(true)
	processed := false
	for range 5 {
		result := h.transactionProcessor.ExecuteTransaction(request)
		if result.Success {
			h.storeTransactionMetrics("default", request)
			processed = true
			break
		}
		//time.Sleep(1 * time.Millisecond)
	}

	if !processed {
		h.transactionProcessor.SetProcessorURL(false)
		result := h.transactionProcessor.ExecuteTransaction(request)
		if result.Success {
			h.storeTransactionMetrics("fallback", request)
			return
		} else {
			// coloca na fila novamente
			h.queueManager.EnqueueTransaction(request)
			return
		}
	}
}
func (h *TransactionHandler) storeTransactionMetrics(processorType string, transaction domain.TransactionRequest) {
	if err := h.metricsRepo.StoreTransactionData(processorType, transaction); err != nil {
		log.Printf("Erro ao armazenar métricas: %v", err)
	}
}

func (h *TransactionHandler) IsQueueAvailable() bool {
	return !h.queueManager.IsQueueFull()
}
