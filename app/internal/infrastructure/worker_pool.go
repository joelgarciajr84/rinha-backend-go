package infrastructure

import (
	"galo/internal/domain"
	"galo/internal/usecase"
)

type WorkerPool struct {
	workerCount        int
	transactionHandler *usecase.TransactionHandler
	queueManager       *ChannelQueueManager
}

func NewWorkerPool(
	workerCount int,
	transactionHandler *usecase.TransactionHandler,
	queueManager *ChannelQueueManager,
) *WorkerPool {
	return &WorkerPool{
		workerCount:        workerCount,
		transactionHandler: transactionHandler,
		queueManager:       queueManager,
	}
}

func (wp *WorkerPool) StartProcessing() {
	transactionQueue := wp.queueManager.GetTransactionQueue()

	for i := 0; i < wp.workerCount; i++ {
		go wp.processTransactions(transactionQueue)
	}
}

func (wp *WorkerPool) processTransactions(transactionQueue <-chan domain.TransactionRequest) {
	for transaction := range transactionQueue {
		wp.transactionHandler.ProcessTransaction(transaction)
	}
}
