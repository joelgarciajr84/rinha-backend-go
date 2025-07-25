package storage

import (
	"context"
	"rinha-go-joel/src/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	summaryQuery = `select processor_type, count(1) as "count", sum(amount) as "total_amount"
			 from financial_transaction
			 where processed_at between $1 and $2
			 group by processor_type;`
)

type TransactionRepository struct {
	dbPool *pgxpool.Pool
}

func NewTransactionRepository(dbPool *pgxpool.Pool) TransactionRepository {
	return TransactionRepository{
		dbPool: dbPool,
	}
}

func (r TransactionRepository) SaveTransaction(ctx context.Context, req domain.TransactionRequest, processorType string) error {
	query := `insert into financial_transaction (correlation_id, amount, processor_type, processed_at)
			  values ($1, $2, $3, $4)
			  on conflict (correlation_id) do nothing`

	_, err := r.dbPool.Exec(ctx, query,
		req.CorrelationID, req.Amount, processorType, req.RequestedAt,
	)

	return err
}

func (r TransactionRepository) TransactionExists(ctx context.Context, correlationID string) (bool, error) {
	query := `select exists(select 1 from financial_transaction where correlation_id = $1)`

	var exists bool
	err := r.dbPool.QueryRow(ctx, query, correlationID).Scan(&exists)

	return exists, err
}

func (r TransactionRepository) GetTransactionSummary(ctx context.Context, from, to string) ([]domain.TransactionSummaryDTO, error) {
	rows, err := r.dbPool.Query(ctx, summaryQuery, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []domain.TransactionSummaryDTO

	for rows.Next() {
		var summary domain.TransactionSummaryDTO
		err := rows.Scan(&summary.ProcessorType, &summary.Count, &summary.TotalAmount)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return summaries, nil
}
