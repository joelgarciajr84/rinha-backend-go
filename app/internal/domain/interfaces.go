package domain

type TransactionProcessor interface {
	ExecuteTransaction(request TransactionRequest, usePrimary bool) ProcessingResult
}

type MetricsRepository interface {
	StoreTransactionData(processorType string, transaction TransactionRequest) error
	RetrieveMetrics(processorType string, timeRange TimeRange) (MetricsData, error)
	ClearAllData() error
}

type QueueManager interface {
	EnqueueTransaction(transaction TransactionRequest) error
	IsQueueFull() bool
}
