package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/AugustoArguello/cacao-settlement-reconciliation/models"
)

type ReportRepo struct {
	mu      sync.RWMutex
	reports map[string]models.ReconciliationReport
}

func NewReportRepo() *ReportRepo {
	return &ReportRepo{
		reports: make(map[string]models.ReconciliationReport),
	}
}

func (r *ReportRepo) Store(_ context.Context, report models.ReconciliationReport) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reports[report.ReportID] = report
	return nil
}

func (r *ReportRepo) GetByID(_ context.Context, id string) (*models.ReconciliationReport, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	report, exists := r.reports[id]
	if !exists {
		return nil, nil
	}
	return &report, nil
}

func (r *ReportRepo) List(_ context.Context) ([]models.ReconciliationReport, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []models.ReconciliationReport
	for _, report := range r.reports {
		results = append(results, report)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].GeneratedAt.After(results[j].GeneratedAt)
	})

	return results, nil
}
