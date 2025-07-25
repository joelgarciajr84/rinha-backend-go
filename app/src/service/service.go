package service

import (
	"context"
	"log"
	"rinha-go-joel/src/domain"
	"time"
)

type (
	TransactionService struct {
		gateway    ProcessorGateway
		repository TransactionRepository
	}

	ProcessorGateway interface {
		ProcessTransaction(ctx context.Context, req domain.TransactionRequest) (string, error)
	}

	TransactionRepository interface {
		SaveTransaction(ctx context.Context, req domain.TransactionRequest, processorType string) error
		GetTransactionSummary(ctx context.Context, from, to string) ([]domain.TransactionSummaryDTO, error)
		TransactionExists(ctx context.Context, correlationID string) (bool, error)
	}
)

func NewTransactionService(gateway ProcessorGateway, repository TransactionRepository) TransactionService {
	return TransactionService{
		gateway:    gateway,
		repository: repository,
	}
}

func (ts TransactionService) ProcessTransaction(ctx context.Context, req domain.TransactionRequest) error {
	req.RequestedAt = time.Now()

	saveCtx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	if exists, err := ts.repository.TransactionExists(saveCtx, req.CorrelationID); err == nil && exists {
		return nil
	}

	processorType, err := ts.gateway.ProcessTransaction(ctx, req)
	if err != nil {
		return err
	}

	saved := false
	for attempt := 1; attempt <= 3 && !saved; attempt++ {
		saveTimeout := time.Duration(attempt*3) * time.Second
		saveCtx, cancel := context.WithTimeout(context.Background(), saveTimeout)

		if err = ts.repository.SaveTransaction(saveCtx, req, processorType); err == nil {
			saved = true
		} else {
		}
		cancel()

		if !saved && attempt < 3 {
			time.Sleep(time.Millisecond * 100)
		}
	}

	if !saved {
		log.Printf("CRITICAL CONSISTENCY ERROR: Payment processor succeeded but failed to save transaction %s after all attempts", req.CorrelationID)
		return nil
	}

	return nil
}

func (ts TransactionService) GetTransactionSummary(ctx context.Context, from, to string) (domain.TransactionSummaryResponse, error) {
	summaries, err := ts.repository.GetTransactionSummary(ctx, from, to)
	if err != nil {
		return domain.TransactionSummaryResponse{}, err
	}

	return domain.BuildSummaryResponse(summaries), nil
}
