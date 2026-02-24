package services

import (
	"context"
	"fmt"

	"github.com/AugustoArguello/cacao-settlement-reconciliation/middleware"
	"github.com/AugustoArguello/cacao-settlement-reconciliation/models"
	"github.com/AugustoArguello/cacao-settlement-reconciliation/repository"
	"github.com/AugustoArguello/cacao-settlement-reconciliation/request"
)

type SettlementService struct {
	repo repository.SettlementRepository
}

func NewSettlementService(repo repository.SettlementRepository) *SettlementService {
	return &SettlementService{repo: repo}
}

func (s *SettlementService) Create(ctx context.Context, req request.CreateSettlementRequest) (*models.SettlementReport, error) {
	if err := req.Validate(); err != nil {
		return nil, middleware.NewValidationError(fmt.Sprintf("validation error: %s", err.Error()))
	}

	report := req.ToModel()
	created, err := s.repo.Store(ctx, report)
	if err != nil {
		return nil, fmt.Errorf("failed to store settlement: %w", err)
	}
	if !created {
		return nil, middleware.NewDuplicateError(fmt.Sprintf("settlement batch %s already exists", report.BatchID))
	}

	return &report, nil
}

func (s *SettlementService) CreateBatch(ctx context.Context, req request.BatchSettlementRequest) (int, int, []request.CreateSettlementRequest, error) {
	var valid []models.SettlementReport
	var invalid []request.CreateSettlementRequest

	for _, stlReq := range req.Settlements {
		if err := stlReq.Validate(); err != nil {
			invalid = append(invalid, stlReq)
			continue
		}
		valid = append(valid, stlReq.ToModel())
	}

	ingested, duplicates, err := s.repo.StoreBatch(ctx, valid)
	if err != nil {
		return 0, 0, invalid, fmt.Errorf("failed to store settlements: %w", err)
	}

	return ingested, duplicates, invalid, nil
}

func (s *SettlementService) GetByBatchID(ctx context.Context, batchID string) (*models.SettlementReport, error) {
	return s.repo.GetByBatchID(ctx, batchID)
}

func (s *SettlementService) List(ctx context.Context, filter repository.SettlementFilter) ([]models.SettlementReport, int, error) {
	return s.repo.List(ctx, filter)
}
