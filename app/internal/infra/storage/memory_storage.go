package storage

import (
	"sync"
	"time"

	"rinha/internal/core/domain"
)

type MemoryStorage struct {
	mu        sync.RWMutex
	processed map[string]struct{}
	payments  []domain.Payment
	stats     map[string]*domain.ProcessorStats
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		processed: make(map[string]struct{}),
		stats: map[string]*domain.ProcessorStats{
			"default":  &domain.ProcessorStats{},
			"fallback": &domain.ProcessorStats{},
		},
		payments: make([]domain.Payment, 0, 10000), // pré-aloca espaço para evitar crescimento contínuo
	}
}

func (m *MemoryStorage) SavePayment(p domain.Payment, processor string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.processed[p.CorrelationID]; exists {
		// Já processado, evita duplicação
		return
	}

	m.processed[p.CorrelationID] = struct{}{}
	m.payments = append(m.payments, p)

	// Atualiza estatísticas
	if stat, ok := m.stats[processor]; ok {
		stat.TotalRequests++
		stat.TotalAmount += p.Amount
	}
}

func (m *MemoryStorage) AlreadyProcessed(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.processed[id]
	return exists
}

// Marca como processado separadamente (caso queira uso explícito)
func (m *MemoryStorage) MarkProcessed(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.processed[id] = struct{}{}
}

func (m *MemoryStorage) GetSummary(from, to *time.Time) domain.Summary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := domain.Summary{
		Default:  domain.ProcessorStats{},
		Fallback: domain.ProcessorStats{},
	}

	for _, p := range m.payments {
		t, err := time.Parse(time.RFC3339Nano, p.RequestedAt)
		if err != nil {
			continue
		}
		if (from == nil || !t.Before(*from)) && (to == nil || !t.After(*to)) {
			processor := "default"
			// Se quiser, a chave do processor pode vir do domain.Payment ou ser passada aqui
			// Exemplo: se CorrelationID começar com F ou outro critério
			if len(p.CorrelationID) > 0 && p.CorrelationID[0] == 'F' {
				processor = "fallback"
			}

			stat := &summary.Default
			if processor == "fallback" {
				stat = &summary.Fallback
			}
			stat.TotalRequests++
			stat.TotalAmount += p.Amount
		}
	}

	return summary
}
