package queue

import (
	"rinha/internal/core/domain"
	"sync"
)

type PaymentTask struct {
	Payment   domain.Payment
	Processor string // "default" ou "fallback"
}

type PaymentQueue struct {
	ch chan PaymentTask
	wg sync.WaitGroup
}

func NewPaymentQueue(buffer int) *PaymentQueue {
	return &PaymentQueue{
		ch: make(chan PaymentTask, buffer),
	}
}

func (q *PaymentQueue) Enqueue(task PaymentTask) {
	q.ch <- task
}

func (q *PaymentQueue) Dequeue() <-chan PaymentTask {
	return q.ch
}

func (q *PaymentQueue) Close() {
	close(q.ch)
	q.wg.Wait()
}
