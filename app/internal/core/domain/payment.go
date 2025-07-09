package domain

type Payment struct {
	CorrelationID string
	Amount        float64
	RequestedAt   string
}

type ProcessorStats struct {
	TotalRequests int
	TotalAmount   float64
}

type Summary struct {
	Default  ProcessorStats
	Fallback ProcessorStats
}
