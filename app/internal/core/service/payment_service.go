package service

import (
	"time"

	"rinha/internal/core/domain"
)

type Storage interface {
	SavePayment(domain.Payment, string)
	AlreadyProcessed(string) bool
	MarkProcessed(string)
	GetSummary(from, to *time.Time) domain.Summary
}

type HealthChecker interface {
	IsHealthy(processor string) bool
}

type PaymentService struct {
	storage       Storage
	healthChecker HealthChecker
	defaultURL    string
	fallbackURL   string
	sender        func(url string, p domain.Payment) error
}

func NewPaymentService(storage Storage, hc HealthChecker, sender func(string, domain.Payment) error, defaultURL, fallbackURL string) *PaymentService {
	return &PaymentService{storage, hc, defaultURL, fallbackURL, sender}
}

func (s *PaymentService) ProcessPayment(p domain.Payment) {
	processor := s.pickProcessor()
	s.sender(processor, p)

	if processor == s.defaultURL && !s.healthChecker.IsHealthy(s.defaultURL) {
		s.sender(s.fallbackURL, p)
	}
}

func (s *PaymentService) pickProcessor() string {
	if s.healthChecker.IsHealthy(s.defaultURL) {
		return s.defaultURL
	}
	return s.fallbackURL
}
