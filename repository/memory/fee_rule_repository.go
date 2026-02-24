package memory

import (
	"context"
	"sync"

	"github.com/AugustoArguello/cacao-settlement-reconciliation/models"
)

type FeeRuleRepo struct {
	mu    sync.RWMutex
	rules map[string]models.FeeRule
}

func NewFeeRuleRepo() *FeeRuleRepo {
	return &FeeRuleRepo{
		rules: make(map[string]models.FeeRule),
	}
}

func (r *FeeRuleRepo) Store(_ context.Context, rule models.FeeRule) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rules[rule.ID] = rule
	return nil
}

func (r *FeeRuleRepo) GetByID(_ context.Context, id string) (*models.FeeRule, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rule, exists := r.rules[id]
	if !exists {
		return nil, nil
	}
	return &rule, nil
}

func (r *FeeRuleRepo) List(_ context.Context, processor *string) ([]models.FeeRule, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []models.FeeRule
	for _, rule := range r.rules {
		if processor != nil && rule.Processor != *processor {
			continue
		}
		results = append(results, rule)
	}
	return results, nil
}

func (r *FeeRuleRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.rules, id)
	return nil
}
