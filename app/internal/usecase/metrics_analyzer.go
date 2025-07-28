package usecase

import (
	"galo/internal/domain"
	"log"
)

type MetricsAnalyzer struct {
	metricsRepo domain.MetricsRepository
}

func NewMetricsAnalyzer(metricsRepo domain.MetricsRepository) *MetricsAnalyzer {
	return &MetricsAnalyzer{
		metricsRepo: metricsRepo,
	}
}

func (a *MetricsAnalyzer) GenerateTransactionReport(timeRange domain.TimeRange) domain.TransactionMetrics {

	primaryMetrics, err := a.metricsRepo.RetrieveMetrics("default", timeRange)
	if err != nil {
		log.Printf("Erro ao recuperar métricas do processador principal: %v", err)
		primaryMetrics = domain.MetricsData{}
	}

	secondaryMetrics, err := a.metricsRepo.RetrieveMetrics("fallback", timeRange)
	if err != nil {
		log.Printf("Erro ao recuperar métricas do processador secundário: %v", err)
		secondaryMetrics = domain.MetricsData{}
	}

	return domain.TransactionMetrics{
		PrimaryProcessor:   primaryMetrics,
		SecondaryProcessor: secondaryMetrics,
	}
}

func (a *MetricsAnalyzer) InitializeSystem() error {
	return a.metricsRepo.ClearAllData()
}
