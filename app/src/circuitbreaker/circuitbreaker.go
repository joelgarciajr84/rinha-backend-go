package circuitbreaker

import (
	"sync"
	"time"
)

type (
	CircuitBreaker struct {
		mu              sync.RWMutex
		failureCount    int
		successCount    int
		lastFailureTime time.Time
		state           State
		maxFailures     int
		timeout         time.Duration
		resetThreshold  int
	}

	State int
)

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

func NewCircuitBreaker(maxFailures int, timeout time.Duration, resetThreshold int) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures:    maxFailures,
		timeout:        timeout,
		resetThreshold: resetThreshold,
		state:          StateClosed,
	}
}

func (cb *CircuitBreaker) CanExecute() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		return time.Since(cb.lastFailureTime) >= cb.timeout
	case StateHalfOpen:
		return true
	default:
		return false
	}
}

func (cb *CircuitBreaker) OnSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.successCount++

	switch cb.state {
	case StateHalfOpen:
		if cb.successCount >= cb.resetThreshold {
			cb.reset()
		}
	case StateOpen:
		cb.state = StateHalfOpen
		cb.successCount = 1
	}
}

func (cb *CircuitBreaker) OnFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount++
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		if cb.failureCount >= cb.maxFailures {
			cb.state = StateOpen
		}
	case StateHalfOpen:
		cb.state = StateOpen
		cb.successCount = 0
	}
}

func (cb *CircuitBreaker) GetState() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

func (cb *CircuitBreaker) reset() {
	cb.failureCount = 0
	cb.successCount = 0
	cb.state = StateClosed
}

func (cb *CircuitBreaker) IsOpen() bool {
	return cb.GetState() == StateOpen && !cb.CanExecute()
}
