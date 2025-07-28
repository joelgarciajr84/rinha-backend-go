package domain

type TransactionProcessor interface {
	ExecuteTransaction(request TransactionRequest) ProcessingResult
}

type ConfigurableProcessor interface {
	TransactionProcessor
	SetProcessorURL(usePrimary bool)
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
