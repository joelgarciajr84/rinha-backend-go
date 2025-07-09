package storage

import (
	"sync"
	"time"

	"rinha/internal/core/domain"
)

type MemoryStorage struct {
	mu        sync.Mutex
	processed map[string]struct{}
	payments  []domain.Payment
	stats     map[string]*domain.ProcessorStats
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		processed: make(map[string]struct{}),
		stats:     map[string]*domain.ProcessorStats{"default": {}, "fallback": {}},
	}
}

func (m *MemoryStorage) SavePayment(p domain.Payment, processor string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.payments = append(m.payments, p)
	s := m.stats[processor]
	s.TotalRequests++
	s.TotalAmount += p.Amount
}

func (m *MemoryStorage) AlreadyProcessed(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, exists := m.processed[id]
	return exists
}

func (m *MemoryStorage) MarkProcessed(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.processed[id] = struct{}{}
}

func (m *MemoryStorage) GetSummary(from, to *time.Time) domain.Summary {
	m.mu.Lock()
	defer m.mu.Unlock()
	summary := domain.Summary{
		Default:  domain.ProcessorStats{},
		Fallback: domain.ProcessorStats{},
	}
	for _, p := range m.payments {
		t, _ := time.Parse(time.RFC3339Nano, p.RequestedAt)
		if (from == nil || !t.Before(*from)) && (to == nil || !t.After(*to)) {
			key := "default"
			if p.CorrelationID[0] == 'F' {
				key = "fallback"
			}
			s := &summary.Default
			if key == "fallback" {
				s = &summary.Fallback
			}
			s.TotalRequests++
			s.TotalAmount += p.Amount
		}
	}
	return summary
}
