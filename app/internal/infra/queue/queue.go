package queue

import (
	"rinha/internal/core/domain"
	"rinha/internal/core/service"
	"sync/atomic"
)

type PaymentQueue struct {
	storage service.Storage
	length  int64
	closed  int32
}

func NewPaymentQueue(storage service.Storage) *PaymentQueue {
	return &PaymentQueue{
		storage: storage,
	}
}

func (q *PaymentQueue) Enqueue(p domain.Payment) error {
	err := q.storage.EnqueuePaymentTask(p)
	if err == nil {
		atomic.AddInt64(&q.length, 1)
	}
	return err
}

func (q *PaymentQueue) Dequeue() <-chan domain.Payment {
	ch := make(chan domain.Payment)

	go func() {
		defer close(ch)
		for {
			p, err := q.storage.DequeuePaymentTask()
			if err != nil {
				// opcional: log do erro e continue tentando
				continue
			}
			if p == nil {
				// fila vazia, espera um pouco para não busy loop
				// pode colocar time.Sleep(50 * time.Millisecond)
				continue
			}
			ch <- *p
			atomic.AddInt64(&q.length, -1)
		}
	}()

	return ch
}

func (q *PaymentQueue) Length() int64 {
	return atomic.LoadInt64(&q.length)
}

func (q *PaymentQueue) Close() {
	// Para Redis não há um close na fila, então apenas marca closed
	atomic.StoreInt32(&q.closed, 1)
}
