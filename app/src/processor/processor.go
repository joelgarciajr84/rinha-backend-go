package processor

import (
	"context"
	"errors"
	"rinha-go-joel/src/domain"
	"rinha-go-joel/src/sentinel"
	"sync"
	"time"
)

type (
	TransactionProcessor struct {
		queue          chan transactionTask
		maxRetries     uint
		retryDelay     time.Duration
		transactionSvc TransactionService
		workers        int
		shutdown       chan struct{}
		wg             sync.WaitGroup
		metrics        *ProcessorMetrics
	}

	TransactionService interface {
		ProcessTransaction(ctx context.Context, req domain.TransactionRequest) error
	}

	transactionTask struct {
		attemptCount uint
		data         domain.TransactionRequest
		submittedAt  time.Time
	}

	ProcessorMetrics struct {
		mu             sync.RWMutex
		Processed      uint64
		Failed         uint64
		Retried        uint64
		QueueSize      int
		AvgProcessTime time.Duration
	}
)

func NewTransactionProcessor(service TransactionService) *TransactionProcessor {
	return &TransactionProcessor{
		transactionSvc: service,
		queue:          make(chan transactionTask, 50000),
		maxRetries:     0,
		retryDelay:     time.Millisecond * 25,
		metrics:        &ProcessorMetrics{},
		workers:        40,
		shutdown:       make(chan struct{}),
	}
}

func (tp *TransactionProcessor) SubmitTransaction(req domain.TransactionRequest) error {
	task := transactionTask{
		attemptCount: 0,
		data:         req,
		submittedAt:  time.Now(),
	}

	select {
	case tp.queue <- task:
		tp.updateQueueSize(len(tp.queue))
		return nil
	default:
		return errors.New("processor queue is full")
	}
}

func (tp *TransactionProcessor) Start() {
	for i := 0; i < tp.workers; i++ {
		tp.wg.Add(1)
		go tp.worker(i)
	}
}

func (tp *TransactionProcessor) Stop() {
	close(tp.shutdown)
	tp.wg.Wait()
}

func (tp *TransactionProcessor) worker(workerID int) {
	defer tp.wg.Done()

	for {
		select {
		case <-tp.shutdown:
			return
		case task := <-tp.queue:
			tp.processTask(task)
			tp.updateQueueSize(len(tp.queue))
		}
	}
}

func (tp *TransactionProcessor) processTask(task transactionTask) {
	startTime := time.Now()
	ctx := context.WithoutCancel(context.Background())

	err := tp.transactionSvc.ProcessTransaction(ctx, task.data)

	processingTime := time.Since(startTime)
	tp.updateAvgProcessTime(processingTime)

	if err != nil {
		if sentinel.IsDismissible(err) {
			tp.incrementFailed()
			return
		}

		if task.attemptCount < tp.maxRetries {
			tp.retryTask(task, err)
			return
		}

		tp.incrementFailed()
		return
	}

	tp.incrementProcessed()
}

func (tp *TransactionProcessor) retryTask(task transactionTask, _ error) {
	task.attemptCount++
	tp.incrementRetried()

	backoffDelay := tp.retryDelay * time.Duration(1<<task.attemptCount)
	if backoffDelay > time.Millisecond*200 {
		backoffDelay = time.Millisecond * 200
	}

	go func() {
		time.Sleep(backoffDelay)
		select {
		case tp.queue <- task:
		case <-tp.shutdown:
		}
	}()
}

func (tp *TransactionProcessor) GetMetrics() ProcessorMetrics {
	tp.metrics.mu.RLock()
	defer tp.metrics.mu.RUnlock()
	return ProcessorMetrics{
		Processed:      tp.metrics.Processed,
		Failed:         tp.metrics.Failed,
		Retried:        tp.metrics.Retried,
		QueueSize:      tp.metrics.QueueSize,
		AvgProcessTime: tp.metrics.AvgProcessTime,
	}
}

func (tp *TransactionProcessor) incrementProcessed() {
	tp.metrics.mu.Lock()
	defer tp.metrics.mu.Unlock()
	tp.metrics.Processed++
}

func (tp *TransactionProcessor) incrementFailed() {
	tp.metrics.mu.Lock()
	defer tp.metrics.mu.Unlock()
	tp.metrics.Failed++
}

func (tp *TransactionProcessor) incrementRetried() {
	tp.metrics.mu.Lock()
	defer tp.metrics.mu.Unlock()
	tp.metrics.Retried++
}

func (tp *TransactionProcessor) updateQueueSize(size int) {
	tp.metrics.mu.Lock()
	defer tp.metrics.mu.Unlock()
	tp.metrics.QueueSize = size
}

func (tp *TransactionProcessor) updateAvgProcessTime(duration time.Duration) {
	tp.metrics.mu.Lock()
	defer tp.metrics.mu.Unlock()

	if tp.metrics.AvgProcessTime == 0 {
		tp.metrics.AvgProcessTime = duration
	} else {
		tp.metrics.AvgProcessTime = (tp.metrics.AvgProcessTime + duration) / 2
	}
}
