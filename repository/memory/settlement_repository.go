package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/AugustoArguello/cacao-settlement-reconciliation/models"
	"github.com/AugustoArguello/cacao-settlement-reconciliation/repository"
)

type SettlementRepo struct {
	mu          sync.RWMutex
	settlements map[string]models.SettlementReport
}

func NewSettlementRepo() *SettlementRepo {
	return &SettlementRepo{
		settlements: make(map[string]models.SettlementReport),
	}
}

func (r *SettlementRepo) Store(_ context.Context, report models.SettlementReport) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.settlements[report.BatchID]; exists {
		return false, nil
	}
	r.settlements[report.BatchID] = report
	return true, nil
}

func (r *SettlementRepo) StoreBatch(_ context.Context, reports []models.SettlementReport) (int, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	ingested, duplicates := 0, 0
	for _, report := range reports {
		if _, exists := r.settlements[report.BatchID]; exists {
			duplicates++
			continue
		}
		r.settlements[report.BatchID] = report
		ingested++
	}
	return ingested, duplicates, nil
}

func (r *SettlementRepo) GetByBatchID(_ context.Context, batchID string) (*models.SettlementReport, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	report, exists := r.settlements[batchID]
	if !exists {
		return nil, nil
	}
	return &report, nil
}

func (r *SettlementRepo) List(_ context.Context, filter repository.SettlementFilter) ([]models.SettlementReport, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []models.SettlementReport
	for _, report := range r.settlements {
		if !matchesSettlementFilter(report, filter) {
			continue
		}
		results = append(results, report)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].SettlementDate.After(results[j].SettlementDate)
	})

	total := len(results)
	start, end := paginate(total, filter.Page, filter.PageSize)
	return results[start:end], total, nil
}

func (r *SettlementRepo) GetByPeriod(_ context.Context, start, end time.Time, processor *string) ([]models.SettlementReport, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []models.SettlementReport
	for _, report := range r.settlements {
		if report.SettlementDate.Before(start) || report.SettlementDate.After(end) {
			continue
		}
		if processor != nil && report.Processor != *processor {
			continue
		}
		results = append(results, report)
	}
	return results, nil
}

func matchesSettlementFilter(report models.SettlementReport, f repository.SettlementFilter) bool {
	if f.Processor != nil && report.Processor != *f.Processor {
		return false
	}
	if f.Currency != nil && report.Currency != *f.Currency {
		return false
	}
	if f.DateFrom != nil && report.SettlementDate.Before(*f.DateFrom) {
		return false
	}
	if f.DateTo != nil && report.SettlementDate.After(*f.DateTo) {
		return false
	}
	return true
}
