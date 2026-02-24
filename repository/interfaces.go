package repository

import (
	"context"
	"time"

	"github.com/AugustoArguello/cacao-settlement-reconciliation/models"
)

type TransactionRepository interface {
	Store(ctx context.Context, txn models.Transaction) (bool, error)
	StoreBatch(ctx context.Context, txns []models.Transaction) (ingested int, duplicates int, err error)
	GetByID(ctx context.Context, id string) (*models.Transaction, error)
	List(ctx context.Context, filter TransactionFilter) ([]models.Transaction, int, error)
	GetByPeriod(ctx context.Context, start, end time.Time, processor *string) ([]models.Transaction, error)
}

type SettlementRepository interface {
	Store(ctx context.Context, report models.SettlementReport) (bool, error)
	StoreBatch(ctx context.Context, reports []models.SettlementReport) (ingested int, duplicates int, err error)
	GetByBatchID(ctx context.Context, batchID string) (*models.SettlementReport, error)
	List(ctx context.Context, filter SettlementFilter) ([]models.SettlementReport, int, error)
	GetByPeriod(ctx context.Context, start, end time.Time, processor *string) ([]models.SettlementReport, error)
}

type DiscrepancyRepository interface {
	Store(ctx context.Context, disc models.Discrepancy) error
	StoreBatch(ctx context.Context, discs []models.Discrepancy) error
	GetByID(ctx context.Context, id string) (*models.Discrepancy, error)
	List(ctx context.Context, filter DiscrepancyFilter) ([]models.Discrepancy, int, error)
}

type ReportRepository interface {
	Store(ctx context.Context, report models.ReconciliationReport) error
	GetByID(ctx context.Context, id string) (*models.ReconciliationReport, error)
	List(ctx context.Context) ([]models.ReconciliationReport, error)
}

type FeeRuleRepository interface {
	Store(ctx context.Context, rule models.FeeRule) error
	GetByID(ctx context.Context, id string) (*models.FeeRule, error)
	List(ctx context.Context, processor *string) ([]models.FeeRule, error)
	Delete(ctx context.Context, id string) error
}

type TransactionFilter struct {
	Processor *string
	Currency  *models.Currency
	Status    *models.TransactionStatus
	Country   *string
	DateFrom  *time.Time
	DateTo    *time.Time
	Page      int
	PageSize  int
}

type SettlementFilter struct {
	Processor *string
	Currency  *models.Currency
	DateFrom  *time.Time
	DateTo    *time.Time
	Page      int
	PageSize  int
}

type DiscrepancyFilter struct {
	Type      *models.DiscrepancyType
	Severity  *models.Severity
	Processor *string
	ReportID  *string
	Page      int
	PageSize  int
}
