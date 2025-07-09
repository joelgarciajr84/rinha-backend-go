package domain

type Payment struct {
	CorrelationID string  `json:"correlationId"`
	Amount        float64 `json:"amount"`
	RequestedAt   string  `json:"requestedAt"`
}

type ProcessorStats struct {
	TotalRequests int     `json:"totalRequests"`
	TotalAmount   float64 `json:"totalAmount"`
}

type Summary struct {
	Default  ProcessorStats `json:"default"`
	Fallback ProcessorStats `json:"fallback"`
}
