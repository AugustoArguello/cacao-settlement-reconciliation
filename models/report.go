package models

import (
	"time"

	"github.com/shopspring/decimal"
)

type DiscrepancySummary struct {
	Type              DiscrepancyType     `json:"type"`
	Count             int                 `json:"count"`
	TotalAmount       decimal.Decimal     `json:"total_amount"`
	SeverityBreakdown map[Severity]int    `json:"severity_breakdown"`
}

type ProcessorSummary struct {
	Processor        string          `json:"processor"`
	TransactionCount int             `json:"transaction_count"`
	ExpectedTotal    decimal.Decimal `json:"expected_total"`
	ActualTotal      decimal.Decimal `json:"actual_total"`
	NetDiscrepancy   decimal.Decimal `json:"net_discrepancy"`
	DiscrepancyCount int             `json:"discrepancy_count"`
}

type SlowSettlement struct {
	TransactionID   string `json:"transaction_id"`
	DaysToSettle    int    `json:"days_to_settle"`
	Processor       string `json:"processor"`
	PaymentMethod   string `json:"payment_method"`
	TransactionDate string `json:"transaction_date"`
	SettlementDate  string `json:"settlement_date"`
	Amount          string `json:"amount"`
}

type ReconciliationReport struct {
	ReportID              string               `json:"report_id"`
	PeriodStart           time.Time            `json:"period_start"`
	PeriodEnd             time.Time            `json:"period_end"`
	GeneratedAt           time.Time            `json:"generated_at"`
	TotalTransactions     int                  `json:"total_transactions"`
	TotalSettlements      int                  `json:"total_settlements"`
	TotalExpected         decimal.Decimal      `json:"total_expected"`
	TotalActual           decimal.Decimal      `json:"total_actual"`
	NetDiscrepancy        decimal.Decimal      `json:"net_discrepancy"`
	DiscrepancyPercentage decimal.Decimal      `json:"discrepancy_percentage"`
	DiscrepancyBreakdown  []DiscrepancySummary `json:"discrepancy_breakdown"`
	ProcessorSummary      []ProcessorSummary   `json:"processor_summary"`
	ProblematicTransactions []Discrepancy      `json:"problematic_transactions"`
	AvgSettlementDays     *decimal.Decimal     `json:"avg_settlement_days,omitempty"`
	SlowSettlements       []SlowSettlement     `json:"slow_settlements,omitempty"`
	Status                string               `json:"status"`
}
