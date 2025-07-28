package infrastructure

import (
	"galo/internal/domain"
)

type ChannelQueueManager struct {
	transactionQueue chan domain.TransactionRequest
	queueCapacity    int
}

func NewChannelQueueManager(queueCapacity int) *ChannelQueueManager {
	return &ChannelQueueManager{
		transactionQueue: make(chan domain.TransactionRequest, queueCapacity),
		queueCapacity:    queueCapacity,
	}
}

func (q *ChannelQueueManager) EnqueueTransaction(transaction domain.TransactionRequest) error {
	select {
	case q.transactionQueue <- transaction:
		return nil
	default:
		return domain.ErrQueueFull
	}
}

func (q *ChannelQueueManager) IsQueueFull() bool {
	return len(q.transactionQueue) >= q.queueCapacity
}

func (q *ChannelQueueManager) GetTransactionQueue() <-chan domain.TransactionRequest {
	return q.transactionQueue
}
