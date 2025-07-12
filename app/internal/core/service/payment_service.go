package service

import (
	"rinha/internal/core/domain"
	"time"
)

type Storage interface {
	SavePayment(p domain.Payment, processor string)
	GetSummary(from, to *time.Time) domain.Summary
	AlreadyProcessed(id string) bool

	// Métodos para fila
	EnqueuePaymentTask(p domain.Payment) error
	DequeuePaymentTask() (*domain.Payment, error)
}

type HealthChecker interface {
	IsHealthy(url string) bool
}

type PaymentService struct {
	storage       Storage
	healthChecker HealthChecker
	sender        SenderFunc
	defaultURL    string
	fallbackURL   string
}

type SenderFunc func(url string, p domain.Payment) error

func NewPaymentService(
	storage Storage,
	healthChecker HealthChecker,
	sender SenderFunc,
	defaultURL, fallbackURL string,
) *PaymentService {
	return &PaymentService{
		storage:       storage,
		healthChecker: healthChecker,
		sender:        sender,
		defaultURL:    defaultURL,
		fallbackURL:   fallbackURL,
	}
}

func (s *PaymentService) ProcessPayment(p domain.Payment) error {
	// Envia para a fila usando Storage (que agora tem esse método)
	return s.storage.EnqueuePaymentTask(p)
}
