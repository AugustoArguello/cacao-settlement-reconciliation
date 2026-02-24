package services

import (
	"context"
	"fmt"

	"github.com/AugustoArguello/cacao-settlement-reconciliation/middleware"
	"github.com/AugustoArguello/cacao-settlement-reconciliation/models"
	"github.com/AugustoArguello/cacao-settlement-reconciliation/repository"
	"github.com/AugustoArguello/cacao-settlement-reconciliation/request"
)

type TransactionService struct {
	repo repository.TransactionRepository
}

func NewTransactionService(repo repository.TransactionRepository) *TransactionService {
	return &TransactionService{repo: repo}
}

func (s *TransactionService) Create(ctx context.Context, req request.CreateTransactionRequest) (*models.Transaction, error) {
	if err := req.Validate(); err != nil {
		return nil, middleware.NewValidationError(fmt.Sprintf("validation error: %s", err.Error()))
	}

	txn := req.ToModel()
	created, err := s.repo.Store(ctx, txn)
	if err != nil {
		return nil, fmt.Errorf("failed to store transaction: %w", err)
	}
	if !created {
		return nil, middleware.NewDuplicateError(fmt.Sprintf("transaction %s already exists", txn.TransactionID))
	}

	return &txn, nil
}

func (s *TransactionService) CreateBatch(ctx context.Context, req request.BatchTransactionRequest) (int, int, []request.CreateTransactionRequest, error) {
	var valid []models.Transaction
	var invalid []request.CreateTransactionRequest

	for _, txnReq := range req.Transactions {
		if err := txnReq.Validate(); err != nil {
			invalid = append(invalid, txnReq)
			continue
		}
		valid = append(valid, txnReq.ToModel())
	}

	ingested, duplicates, err := s.repo.StoreBatch(ctx, valid)
	if err != nil {
		return 0, 0, invalid, fmt.Errorf("failed to store transactions: %w", err)
	}

	return ingested, duplicates, invalid, nil
}

func (s *TransactionService) GetByID(ctx context.Context, id string) (*models.Transaction, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *TransactionService) List(ctx context.Context, filter repository.TransactionFilter) ([]models.Transaction, int, error) {
	return s.repo.List(ctx, filter)
}
