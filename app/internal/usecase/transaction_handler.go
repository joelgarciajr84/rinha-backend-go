package usecase

import (
	"galo/internal/domain"
	"log"
	"math/rand"
	"time"
)

type TransactionHandler struct {
	primaryProcessorURL  string
	fallbackProcessorURL string
	metricsRepo          domain.MetricsRepository
	transactionProcessor domain.TransactionProcessor
	queueManager         domain.QueueManager
}

// Parâmetros de retry
const (
	defaultMaxRetries  = 5                    // tentativas no default
	defaultSleepBase   = 3 * time.Millisecond // base do backoff
	fallbackMaxRetries = 2                    // tentativas no fallback
	fallbackSleepBase  = 3 * time.Millisecond
	jitterRangeMicros  = 500 // jitter ±500µs
)

func NewTransactionHandler(
	primaryURL, fallbackURL string,
	metricsRepo domain.MetricsRepository,
	processor domain.TransactionProcessor,
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
	// Timestamp único e de alta precisão — mesmo valor usado no PP e no Redis
	request.SubmittedAt = time.Now().UTC().Format(time.RFC3339Nano)

	defer func() {
		if r := recover(); r != nil {
			log.Printf("panic ao processar transação %s: %v", request.IdentificationCode, r)
		}
	}()

	// 1) Tenta no DEFAULT com pequenos retries
	if h.tryProcessAndStore(request, true, defaultMaxRetries, defaultSleepBase) {
		return
	}

	// 2) Cai para o FALLBACK (1..2 tentativas curtas)
	_ = h.tryProcessAndStore(request, false, fallbackMaxRetries, fallbackSleepBase)
}

// tryProcessAndStore tenta processar N vezes no alvo indicado e, se sucesso,
// persiste a métrica uma única vez.
func (h *TransactionHandler) tryProcessAndStore(
	request domain.TransactionRequest,
	usePrimary bool,
	maxRetries int,
	sleepBase time.Duration,
) bool {
	for attempt := 0; attempt < maxRetries; attempt++ {
		result := h.transactionProcessor.ExecuteTransaction(request, usePrimary)
		if result.Success {
			if usePrimary {
				h.storeTransactionMetrics("default", request)
			} else {
				h.storeTransactionMetrics("fallback", request)
			}
			return true
		}

		// backoff curtinho com jitter para evitar thundering herd
		if attempt < maxRetries-1 {
			jitter := time.Duration(rand.Intn(jitterRangeMicros)*int(time.Microsecond)) - (jitterRangeMicros/2)*time.Microsecond
			time.Sleep(sleepBase + jitter)
		}
	}
	return false
}

func (h *TransactionHandler) storeTransactionMetrics(processorType string, transaction domain.TransactionRequest) {
	// Assumindo StoreTransactionData idempotente (guard SETNX no Redis)
	if err := h.metricsRepo.StoreTransactionData(processorType, transaction); err != nil {
		log.Printf("Erro ao armazenar métricas: %v", err)
	}
}

func (h *TransactionHandler) IsQueueAvailable() bool {
	return !h.queueManager.IsQueueFull()
}
