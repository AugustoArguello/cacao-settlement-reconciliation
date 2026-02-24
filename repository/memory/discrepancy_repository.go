package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/AugustoArguello/cacao-settlement-reconciliation/models"
	"github.com/AugustoArguello/cacao-settlement-reconciliation/repository"
)

type DiscrepancyRepo struct {
	mu            sync.RWMutex
	discrepancies map[string]models.Discrepancy
}

func NewDiscrepancyRepo() *DiscrepancyRepo {
	return &DiscrepancyRepo{
		discrepancies: make(map[string]models.Discrepancy),
	}
}

func (r *DiscrepancyRepo) Store(_ context.Context, disc models.Discrepancy) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.discrepancies[disc.ID] = disc
	return nil
}

func (r *DiscrepancyRepo) StoreBatch(_ context.Context, discs []models.Discrepancy) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, disc := range discs {
		r.discrepancies[disc.ID] = disc
	}
	return nil
}

func (r *DiscrepancyRepo) GetByID(_ context.Context, id string) (*models.Discrepancy, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	disc, exists := r.discrepancies[id]
	if !exists {
		return nil, nil
	}
	return &disc, nil
}

func (r *DiscrepancyRepo) List(_ context.Context, filter repository.DiscrepancyFilter) ([]models.Discrepancy, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []models.Discrepancy
	for _, disc := range r.discrepancies {
		if !matchesDiscrepancyFilter(disc, filter) {
			continue
		}
		results = append(results, disc)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].DetectedAt.After(results[j].DetectedAt)
	})

	total := len(results)
	start, end := paginate(total, filter.Page, filter.PageSize)
	return results[start:end], total, nil
}

func matchesDiscrepancyFilter(disc models.Discrepancy, f repository.DiscrepancyFilter) bool {
	if f.Type != nil && disc.Type != *f.Type {
		return false
	}
	if f.Severity != nil && disc.Severity != *f.Severity {
		return false
	}
	if f.Processor != nil && (disc.Processor == nil || *disc.Processor != *f.Processor) {
		return false
	}
	return true
}
