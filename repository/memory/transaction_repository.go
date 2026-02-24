package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/AugustoArguello/cacao-settlement-reconciliation/models"
	"github.com/AugustoArguello/cacao-settlement-reconciliation/repository"
)

type TransactionRepo struct {
	mu           sync.RWMutex
	transactions map[string]models.Transaction
}

func NewTransactionRepo() *TransactionRepo {
	return &TransactionRepo{
		transactions: make(map[string]models.Transaction),
	}
}

func (r *TransactionRepo) Store(_ context.Context, txn models.Transaction) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.transactions[txn.TransactionID]; exists {
		return false, nil
	}
	r.transactions[txn.TransactionID] = txn
	return true, nil
}

func (r *TransactionRepo) StoreBatch(_ context.Context, txns []models.Transaction) (int, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	ingested, duplicates := 0, 0
	for _, txn := range txns {
		if _, exists := r.transactions[txn.TransactionID]; exists {
			duplicates++
			continue
		}
		r.transactions[txn.TransactionID] = txn
		ingested++
	}
	return ingested, duplicates, nil
}

func (r *TransactionRepo) GetByID(_ context.Context, id string) (*models.Transaction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	txn, exists := r.transactions[id]
	if !exists {
		return nil, nil
	}
	return &txn, nil
}

func (r *TransactionRepo) List(_ context.Context, filter repository.TransactionFilter) ([]models.Transaction, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []models.Transaction
	for _, txn := range r.transactions {
		if !matchesTransactionFilter(txn, filter) {
			continue
		}
		results = append(results, txn)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.After(results[j].Timestamp)
	})

	total := len(results)
	start, end := paginate(total, filter.Page, filter.PageSize)
	return results[start:end], total, nil
}

func (r *TransactionRepo) GetByPeriod(_ context.Context, start, end time.Time, processor *string) ([]models.Transaction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []models.Transaction
	for _, txn := range r.transactions {
		txnDate := txn.Timestamp
		if txnDate.Before(start) || txnDate.After(end) {
			continue
		}
		if processor != nil && txn.Processor != *processor {
			continue
		}
		results = append(results, txn)
	}
	return results, nil
}

func matchesTransactionFilter(txn models.Transaction, f repository.TransactionFilter) bool {
	if f.Processor != nil && txn.Processor != *f.Processor {
		return false
	}
	if f.Currency != nil && txn.Currency != *f.Currency {
		return false
	}
	if f.Status != nil && txn.Status != *f.Status {
		return false
	}
	if f.Country != nil && txn.CustomerCountry != *f.Country {
		return false
	}
	if f.DateFrom != nil && txn.Timestamp.Before(*f.DateFrom) {
		return false
	}
	if f.DateTo != nil && txn.Timestamp.After(*f.DateTo) {
		return false
	}
	return true
}
